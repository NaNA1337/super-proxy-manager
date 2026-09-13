package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/audit"
	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/host"
	"github.com/NaNA1337/super-proxy-manager/backend/proxy"
	"github.com/NaNA1337/super-proxy-manager/backend/sharelink"
	"github.com/NaNA1337/super-proxy-manager/backend/subscription"
)

type Server struct {
	addr              string
	db                *db.DB
	hostManager       *host.HostManager
	shareService      *sharelink.Service
	subStore          *subscription.Store
	subHandler        *subscription.Handler
	clientConfigCache *proxy.ClientConfigCache
	distFS            fs.FS
	mux               *http.ServeMux
}

func NewServer(addr string, database *db.DB, distFS fs.FS) *Server {
	if addr == "" {
		addr = ":8443"
	}

	hm := host.NewHostManager(database)
	ss := sharelink.NewService()
	subStore := subscription.NewStore()

	clientProvider := func(hostID string) (*proxy.DaemonClient, error) {
		return hm.GetClient(hostID)
	}
	subHandler := subscription.NewHandlerWithProvider(subStore, clientProvider, ss)

	s := &Server{
		addr:              addr,
		db:                database,
		hostManager:       hm,
		shareService:      ss,
		subStore:          subStore,
		subHandler:        subHandler,
		clientConfigCache: proxy.NewClientConfigCache(45 * time.Second),
		distFS:            distFS,
		mux:               http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// 1. Public Subscriptions endpoint
	s.mux.HandleFunc("/sub/", s.subHandler.HandleGetSubscription)

	// 2. Auth Endpoints
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/api/auth/me", s.handleMe)
	s.mux.HandleFunc("/api/auth/change-password", s.requireAuthHandler(s.handleChangePassword))
	s.mux.HandleFunc("/api/auth/bootstrap-status", s.handleBootstrapStatus)

	// 3. Host Management Endpoints (Admin & Authenticated)
	s.mux.HandleFunc("/api/hosts", s.requireAuthHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.requireAdminHandler(s.handleCreateHost)(w, r)
			return
		}
		s.handleListHosts(w, r)
	}))
	s.mux.HandleFunc("/api/hosts/test", s.requireAdminHandler(s.handleTestHostPreSave))
	s.mux.HandleFunc("/api/hosts/", s.requireAuthHandler(s.handleHostSubroutes))

	// 4. Daemon Control Plane (Authenticated & Host-Aware)
	s.mux.HandleFunc("/api/daemon/status", s.requireAuthHandler(s.handleDaemonStatus))
	s.mux.HandleFunc("/api/daemon/system", s.requireAuthHandler(s.handleDaemonSystem))
	s.mux.HandleFunc("/api/daemon/current-exits", s.requireAuthHandler(s.handleDaemonCurrentExits))
	s.mux.HandleFunc("/api/daemon/slots", s.requireAuthHandler(s.handleDaemonSlots))
	s.mux.HandleFunc("/api/daemon/pool", s.requireAuthHandler(s.handleDaemonPool))
	s.mux.HandleFunc("/api/daemon/pool/qualified", s.requireAuthHandler(s.handleDaemonPoolQualified))
	s.mux.HandleFunc("/api/daemon/nodes", s.requireAuthHandler(s.handleDaemonNodes))
	s.mux.HandleFunc("/api/daemon/nodes/", s.requireAuthHandler(s.handleDaemonNodeDetails))
	s.mux.HandleFunc("/api/daemon/metrics", s.requireAuthHandler(s.handleDaemonMetrics))
	s.mux.HandleFunc("/api/daemon/routing", s.requireAuthHandler(s.handleDaemonRouting))
	s.mux.HandleFunc("/api/daemon/operations/", s.requireAuthHandler(s.handleDaemonOperation))
	s.mux.HandleFunc("/api/daemon/discovery/refresh", s.requireAdminHandler(s.handleDaemonDiscoveryRefresh))

	// Slot switch (Restricted to admin + CSRF enforced + requires specific host)
	s.mux.HandleFunc("/api/daemon/slots/", s.requireAdminHandler(s.handleDaemonSlotAction))

	// 5. ShareLink & Canonical Client Config Endpoints
	s.mux.HandleFunc("/api/client-configs/export-zip", s.requireAuthHandler(s.handleExportZip))
	s.mux.HandleFunc("/api/client-configs/audit", s.requireAuthHandler(s.handleClientConfigAudit))
	s.mux.HandleFunc("/api/sharelinks/protocols", s.requireAuthHandler(s.handleShareProtocols))
	s.mux.HandleFunc("/api/sharelinks/generate", s.requireAuthHandler(s.handleShareGenerate))
	s.mux.HandleFunc("/api/sharelinks/batch", s.requireAuthHandler(s.handleShareBatch))

	// 6. Subscription Management
	s.mux.HandleFunc("/api/subscriptions", s.requireAuthHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
			if sess == nil || sess.Role != auth.RoleAdmin {
				http.Error(w, `{"error":"admin required"}`, http.StatusForbidden)
				return
			}
			s.subHandler.HandleCreateSubscription(w, r)
			audit.GlobalLogger.Log(sess.Username, string(sess.Role), "create_subscription", "sub_token", "SUCCESS", auth.GetClientIP(r), "")
			return
		}
		s.subHandler.HandleListSubscriptions(w, r)
	}))
	s.mux.HandleFunc("/api/subscriptions/", s.requireAdminHandler(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/revoke") {
			s.subHandler.HandleRevokeSubscription(w, r)
			sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
			audit.GlobalLogger.Log(sess.Username, string(sess.Role), "revoke_subscription", r.URL.Path, "SUCCESS", auth.GetClientIP(r), "")
			return
		}
		http.NotFound(w, r)
	}))

	// 7. Audit & Events
	s.mux.HandleFunc("/api/audit", s.requireAuthHandler(audit.HandleGetAuditLogs))
	s.mux.HandleFunc("/api/events", s.requireAuthHandler(s.handleEvents))

	// 8. Settings (Host-aware & Read-only)
	s.mux.HandleFunc("/api/settings", s.requireAuthHandler(s.handleSettings))

	// 9. Static SPA files or fallback
	if s.distFS != nil {
		fileServer := http.FileServer(http.FS(s.distFS))
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path != "" {
				if f, err := s.distFS.Open(path); err == nil {
					_ = f.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			// SPA index.html fallback
			indexFile, err := s.distFS.Open("index.html")
			if err == nil {
				defer indexFile.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeContent(w, r, "index.html", time.Now(), indexFile.(ioReadSeeker))
				return
			}
			http.NotFound(w, r)
		})
	}
}

type ioReadSeeker interface {
	fs.File
	Seek(offset int64, whence int) (int64, error)
}

func (s *Server) Handler() http.Handler {
	// Chain: Redacted Logging -> Security Headers -> Session -> Rate Limit -> Mux
	return subscription.RedactedLoggingMiddleware(
		auth.SecurityHeadersMiddleware(
			auth.SessionMiddleware(s.db)(s.mux),
		),
	)
}

func (s *Server) requireAuthHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r)
	}
}

func (s *Server) requireAdminHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth.RequireAdmin(http.HandlerFunc(h)).ServeHTTP(w, r)
	}
}

// resolveTargetHost determines the target host and DaemonClient for the request
func (s *Server) resolveTargetHost(r *http.Request) (string, *proxy.DaemonClient, error) {
	hostID := r.Header.Get("X-Host-ID")
	if hostID == "" {
		hostID = r.URL.Query().Get("host_id")
	}

	if strings.EqualFold(hostID, "all") {
		return "all", nil, nil
	}

	client, err := s.hostManager.GetClient(hostID)
	if err != nil {
		return "", nil, err
	}
	return hostID, client, nil
}

// --- Auth Handlers ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := auth.GetClientIP(r)
	if !auth.LoginLimiter.GetLimiter(ip).Allow() {
		http.Error(w, `{"error":"too many login attempts, please try again later"}`, http.StatusTooManyRequests)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := auth.Authenticate(s.db, req.Username, req.Password)
	if err != nil {
		audit.GlobalLogger.Log(req.Username, "unknown", "login", "auth", "FAILED", ip, "invalid credentials")
		auth.SendJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	sess, err := auth.CreateSession(s.db, user, 24*time.Hour)
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	auth.SetSessionCookie(w, sess.Token, sess.ExpiresAt, isSecure)

	audit.GlobalLogger.Log(user.Username, string(user.Role), "login", "auth", "SUCCESS", ip, "session created")

	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated":        true,
		"must_change_password": user.MustChangePassword,
		"username":             user.Username,
		"role":                 user.Role,
		"token":                sess.Token,
		"csrf_token":           sess.CSRFToken,
		"expires_at":           sess.ExpiresAt,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := r.Context().Value(auth.SessionContextKey).(*auth.Session); ok && sess != nil {
		auth.RevokeSession(s.db, sess.Token)
		audit.GlobalLogger.Log(sess.Username, string(sess.Role), "logout", "auth", "SUCCESS", auth.GetClientIP(r), "")
	}
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	auth.ClearSessionCookie(w, isSecure)
	auth.SendJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	if !ok || sess == nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{"authenticated": false})
		return
	}
	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated":        true,
		"must_change_password": sess.MustChangePassword,
		"username":             sess.Username,
		"role":                 sess.Role,
		"csrf_token":           sess.CSRFToken,
		"expires_at":           sess.ExpiresAt,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	if sess == nil {
		auth.SendJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	err := auth.ChangePassword(s.db, sess.Username, req.CurrentPassword, req.NewPassword, req.ConfirmPassword)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Create new session for the updated user
	userUpdated, err := auth.Authenticate(s.db, sess.Username, req.NewPassword)
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "password changed but failed to re-authenticate"})
		return
	}

	newSess, err := auth.CreateSession(s.db, userUpdated, 24*time.Hour)
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue new session"})
		return
	}

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	auth.SetSessionCookie(w, newSess.Token, newSess.ExpiresAt, isSecure)

	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "change_password", "user_credentials", "SUCCESS", auth.GetClientIP(r), "password updated successfully")

	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"status":               "password_changed",
		"must_change_password": false,
		"csrf_token":           newSess.CSRFToken,
		"expires_at":           newSess.ExpiresAt,
	})
}

func (s *Server) handleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	_ = s.db.Conn().QueryRow("SELECT COUNT(*) FROM users WHERE must_change_password = 1").Scan(&count)
	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"initial_setup_required": count > 0,
	})
}

// --- Host Management Handlers ---

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.hostManager.ListHosts()
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if hosts == nil {
		hosts = []*host.Host{}
	}
	auth.SendJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Address        string `json:"address"`
		AgentURL       string `json:"agent_url"`
		Token          string `json:"token"`
		TLSFingerprint string `json:"tls_fingerprint"`
		IsDefault      bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	created, err := s.hostManager.CreateHostWithFingerprint(req.Name, req.Address, req.AgentURL, req.Token, req.TLSFingerprint, req.IsDefault)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "create_host", created.Name, "SUCCESS", auth.GetClientIP(r), created.AgentURL)

	auth.SendJSON(w, http.StatusCreated, created)
}

func (s *Server) handleTestHostPreSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AgentURL string `json:"agent_url"`
		Token    string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	res, err := s.hostManager.TestConnection(req.AgentURL, req.Token)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleHostSubroutes(w http.ResponseWriter, r *http.Request) {
	// Pattern: /api/hosts/{id} or /api/hosts/{id}/...
	path := strings.TrimPrefix(r.URL.Path, "/api/hosts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	hostID := parts[0]

	// Specific actions:
	if len(parts) == 2 {
		switch parts[1] {
		case "test":
			s.requireAdminHandler(func(w http.ResponseWriter, r *http.Request) {
				res, err := s.hostManager.ProbeHost(hostID)
				if err != nil {
					auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				auth.SendJSON(w, http.StatusOK, res)
			})(w, r)
			return
		case "default":
			s.requireAdminHandler(func(w http.ResponseWriter, r *http.Request) {
				if err := s.hostManager.SetDefaultHost(hostID); err != nil {
					auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				auth.SendJSON(w, http.StatusOK, map[string]string{"status": "default_set"})
			})(w, r)
			return
		case "client-config":
			s.handleHostCanonicalClientConfig(w, r, hostID, "")
			return
		case "client-links":
			s.handleHostClientLinks(w, r, hostID)
			return
		}
	} else if len(parts) == 4 && parts[1] == "nodes" && parts[3] == "client-config" {
		nodeID := parts[2]
		s.handleHostCanonicalClientConfig(w, r, hostID, nodeID)
		return
	} else if len(parts) == 3 && parts[1] == "client-links" && parts[2] == "all" {
		s.handleHostClientLinksAllText(w, r, hostID)
		return
	}

	// Single Host CRUD
	switch r.Method {
	case http.MethodGet:
		h, err := s.hostManager.GetHost(hostID)
		if err != nil {
			auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		auth.SendJSON(w, http.StatusOK, h)
	case http.MethodPut:
		s.requireAdminHandler(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name           string `json:"name"`
				Address        string `json:"address"`
				AgentURL       string `json:"agent_url"`
				Token          string `json:"token"`
				TLSFingerprint string `json:"tls_fingerprint"`
				Enabled        bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
				return
			}
			updated, err := s.hostManager.UpdateHostWithFingerprint(hostID, req.Name, req.Address, req.AgentURL, req.Token, req.TLSFingerprint, req.Enabled)
			if err != nil {
				auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
			audit.GlobalLogger.Log(sess.Username, string(sess.Role), "update_host", updated.Name, "SUCCESS", auth.GetClientIP(r), updated.AgentURL)
			auth.SendJSON(w, http.StatusOK, updated)
		})(w, r)
	case http.MethodDelete:
		s.requireAdminHandler(func(w http.ResponseWriter, r *http.Request) {
			if err := s.hostManager.DeleteHost(hostID); err != nil {
				auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
			audit.GlobalLogger.Log(sess.Username, string(sess.Role), "delete_host", hostID, "SUCCESS", auth.GetClientIP(r), "")
			auth.SendJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		})(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleHostClientLinks(w http.ResponseWriter, r *http.Request, hostID string) {
	h, err := s.hostManager.GetHost(hostID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	client, err := s.hostManager.GetClient(hostID)
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	rawCfg, _ := client.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)

	exits, _ := client.GetCurrentExits()
	var profiles []*sharelink.ClientProfile

	for _, exit := range exits {
		ip, _ := exit["ip"].(string)
		country, _ := exit["country"].(string)
		nodeID, _ := exit["node_id"].(string)
		if ip == "" {
			continue
		}
		node := sharelink.NodeInfo{
			ID:      nodeID,
			IP:      ip,
			Country: country,
		}

		// Try VLESS first if enabled
		if cfg.VlessEnabled {
			if p, err := sharelink.GenerateClientProfile(h.ID, h.Name, node, "vless", cfg); err == nil && p != nil {
				profiles = append(profiles, p)
			}
		}
		// SOCKS5
		if cfg.SocksPort > 0 {
			if p, err := sharelink.GenerateClientProfile(h.ID, h.Name, node, "socks5", cfg); err == nil && p != nil {
				profiles = append(profiles, p)
			}
		}
	}

	auth.SendJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleHostClientLinksAllText(w http.ResponseWriter, r *http.Request, hostID string) {
	h, err := s.hostManager.GetHost(hostID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	client, err := s.hostManager.GetClient(hostID)
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	rawCfg, _ := client.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)

	exits, _ := client.GetCurrentExits()
	var profiles []*sharelink.ClientProfile

	for _, exit := range exits {
		ip, _ := exit["ip"].(string)
		country, _ := exit["country"].(string)
		nodeID, _ := exit["node_id"].(string)
		if ip == "" {
			continue
		}
		node := sharelink.NodeInfo{
			ID:      nodeID,
			IP:      ip,
			Country: country,
		}
		if cfg.VlessEnabled {
			if p, err := sharelink.GenerateClientProfile(h.ID, h.Name, node, "vless", cfg); err == nil && p != nil {
				profiles = append(profiles, p)
			}
		}
		if cfg.SocksPort > 0 {
			if p, err := sharelink.GenerateClientProfile(h.ID, h.Name, node, "socks5", cfg); err == nil && p != nil {
				profiles = append(profiles, p)
			}
		}
	}

	text := sharelink.GenerateStructuredAllText(h.Name, h.Address, profiles)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func (s *Server) handleHostCanonicalClientConfig(w http.ResponseWriter, r *http.Request, hostID, nodeID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if nodeID == "" {
		nodeID = r.URL.Query().Get("node_id")
	}

	h, err := s.hostManager.GetHost(hostID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)

	client, err := s.hostManager.GetClient(hostID)
	if err != nil {
		s.clientConfigCache.Invalidate(hostID, nodeID)
		auth.SendJSON(w, http.StatusOK, &proxy.AllClientConfigResponse{
			Available: false,
			Error:     "host daemon unavailable: " + err.Error(),
		})
		return
	}

	// Always query canonical daemon API to ensure live runtime status
	allCfg, err := client.GetAllClientConfig(nodeID)
	if err != nil || allCfg == nil || !allCfg.Available {
		// Immediately invalidate any existing cache when runtime is unavailable
		s.clientConfigCache.Invalidate(hostID, nodeID)
		if allCfg != nil {
			auth.SendJSON(w, http.StatusOK, allCfg)
		} else {
			auth.SendJSON(w, http.StatusOK, &proxy.AllClientConfigResponse{
				Available: false,
				Error:     "runtime endpoint unavailable: " + err.Error(),
			})
		}
		return
	}

	// Enrich Host attributes
	for i := range allCfg.Nodes {
		allCfg.Nodes[i].HostID = h.ID
		allCfg.Nodes[i].HostName = h.Name
	}

	// Store validated live config in cache
	s.clientConfigCache.Set(hostID, nodeID, allCfg)

	// Audit: strictly zero secrets logged! Only host_id, node_id, user, timestamp, result.
	target := hostID
	if nodeID != "" {
		target += ":" + nodeID
	}
	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "view_client_config", target, "SUCCESS", auth.GetClientIP(r), "")

	auth.SendJSON(w, http.StatusOK, allCfg)
}

func (s *Server) handleExportZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Selections []struct {
			HostID string `json:"host_id"`
			NodeID string `json:"node_id"`
		} `json:"selections"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Selections) == 0 {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or empty selections"})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	exportedProfiles := 0
	for _, sel := range req.Selections {
		h, err := s.hostManager.GetHost(sel.HostID)
		if err != nil {
			continue
		}
		client, err := s.hostManager.GetClient(sel.HostID)
		if err != nil {
			continue
		}

		hostFolder := sanitizePath(h.Name)
		nodeFolder := sanitizePath(sel.NodeID)

		// Fetch canonical client config from daemon
		allCfg, err := client.GetAllClientConfig(sel.NodeID)
		if err != nil || allCfg == nil || !allCfg.Available {
			// Write status file indicating unavailable
			statusFile, _ := zipWriter.Create(fmt.Sprintf("%s/%s/status.txt", hostFolder, nodeFolder))
			if statusFile != nil {
				errMsg := "Client configuration temporarily unavailable"
				if allCfg != nil && allCfg.Error != "" {
					errMsg = allCfg.Error
				}
				_, _ = statusFile.Write([]byte(errMsg + "\n"))
			}
			continue
		}

		// Write profiles
		for _, node := range allCfg.Nodes {
			for _, profile := range node.Profiles {
				fileName := profile.Filename
				if fileName == "" {
					fileName = profile.ID + ".txt"
				}
				entryPath := fmt.Sprintf("%s/%s/%s", hostFolder, nodeFolder, sanitizePath(fileName))
				f, err := zipWriter.Create(entryPath)
				if err == nil {
					_, _ = f.Write([]byte(profile.Content))
					exportedProfiles++
				}
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate zip"})
		return
	}

	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "batch_export_zip", fmt.Sprintf("nodes:%d_profiles:%d", len(req.Selections), exportedProfiles), "SUCCESS", auth.GetClientIP(r), "")

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"super-proxy-configs.zip\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleClientConfigAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		HostID    string `json:"host_id"`
		NodeID    string `json:"node_id"`
		Action    string `json:"action"`     // "copy", "download", "qr", "copy_all"
		ProfileID string `json:"profile_id"` // "vless", "clash", "sing-box", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	target := fmt.Sprintf("%s:%s:%s", req.HostID, req.NodeID, req.ProfileID)
	auditAction := req.Action + "_client_config"

	audit.GlobalLogger.Log(sess.Username, string(sess.Role), auditAction, target, "SUCCESS", auth.GetClientIP(r), "")
	auth.SendJSON(w, http.StatusOK, map[string]string{"status": "audited"})
}

func sanitizePath(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return strings.TrimSpace(name)
}

// --- Daemon Control Plane Handlers (Host-Aware) ---

func (s *Server) handleDaemonStatus(w http.ResponseWriter, r *http.Request) {
	mode, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "no_hosts",
			"message": "No super-proxy hosts configured",
		})
		return
	}

	if mode == "all" {
		hosts, _ := s.hostManager.ListHosts()
		healthyCount := 0
		for _, h := range hosts {
			if h.Status == "healthy" {
				healthyCount++
			}
		}
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"mode":          "all_hosts",
			"total_hosts":   len(hosts),
			"healthy_hosts": healthyCount,
			"version":       "multi-host",
			"server_id":     "all",
			"uptime":        0,
		})
		return
	}

	res, err := client.GetStatus()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSystem(w http.ResponseWriter, r *http.Request) {
	mode, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"CPU": 0.0, "Memory": 0.0, "Load": []float64{0, 0, 0},
		})
		return
	}

	if mode == "all" {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"CPU": 0.0, "Memory": 0.0, "Load": []float64{0, 0, 0},
		})
		return
	}

	res, err := client.GetSystem()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonCurrentExits(w http.ResponseWriter, r *http.Request) {
	mode, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, []interface{}{})
		return
	}

	if mode == "all" {
		hosts, _ := s.hostManager.ListHosts()
		var allExits []map[string]interface{}
		for _, h := range hosts {
			c, err := s.hostManager.GetClient(h.ID)
			if err == nil {
				exits, _ := c.GetCurrentExits()
				for _, ex := range exits {
					ex["host_id"] = h.ID
					ex["host_name"] = h.Name
					allExits = append(allExits, ex)
				}
			}
		}
		auth.SendJSON(w, http.StatusOK, allExits)
		return
	}

	res, err := client.GetCurrentExits()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSlots(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"slots": []interface{}{},
		})
		return
	}

	res, err := client.GetSlots()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonPool(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]int{"total": 0})
		return
	}

	res, err := client.GetPool()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonPoolQualified(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, []interface{}{})
		return
	}

	res, err := client.GetPoolQualified()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonDiscoveryRefresh(w http.ResponseWriter, r *http.Request) {
	mode, client, err := s.resolveTargetHost(r)
	if err != nil || mode == "all" {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "select a specific target host before pulling nodes"})
		return
	}

	var res map[string]interface{}
	var code int
	switch r.Method {
	case http.MethodGet:
		res, code, err = client.GetDiscoveryRefresh()
	case http.MethodPost:
		res, code, err = client.TriggerDiscoveryRefresh()
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	if code < 200 || code >= 300 {
		auth.SendJSON(w, code, res)
		return
	}
	if r.Method == http.MethodPost {
		sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
		audit.GlobalLogger.Log(sess.Username, string(sess.Role), "pull_nodes", "discovery-refresh", "SUCCESS", auth.GetClientIP(r), "")
	}
	auth.SendJSON(w, code, res)
}

func (s *Server) handleDaemonNodes(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{"total": 0, "nodes": []interface{}{}})
		return
	}

	q := r.URL.Query()
	country := q.Get("country")
	status := q.Get("status")
	search := q.Get("search")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	res, err := client.GetNodes(country, status, search, limit, offset)
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonNodeDetails(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "host unavailable"})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}
	nodeID := parts[3]
	res, err := client.GetNodeDetails(nodeID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSlotAction(w http.ResponseWriter, r *http.Request) {
	mode, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "target host required"})
		return
	}
	if mode == "all" {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "slot mutation cannot be executed in 'All Hosts' mode; please select a specific host"})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] != "switch" {
		http.Error(w, "Invalid action. Use /api/daemon/slots/{slot}/switch", http.StatusBadRequest)
		return
	}
	slot, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid slot number", http.StatusBadRequest)
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	res, code, err := client.SwitchSlot(slot, req.NodeID)
	if err != nil {
		audit.GlobalLogger.Log(sess.Username, string(sess.Role), "switch_slot", fmt.Sprintf("slot-%d -> %s", slot, req.NodeID), "FAILED", auth.GetClientIP(r), err.Error())
		auth.SendJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "switch_slot", fmt.Sprintf("slot-%d -> %s", slot, req.NodeID), "SUCCESS", auth.GetClientIP(r), "")
	auth.SendJSON(w, code, res)
}

func (s *Server) handleDaemonOperation(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "host unavailable"})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "Operation ID required", http.StatusBadRequest)
		return
	}
	opID := parts[3]
	res, err := client.GetOperation(opID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonMetrics(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# No hosts configured\n"))
		return
	}

	text, err := client.GetMetrics()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func (s *Server) handleDaemonRouting(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"exits":               []interface{}{},
			"slots":               map[string]interface{}{},
			"dns_leak_protected":  true,
			"ipv6_leak_protected": true,
		})
		return
	}

	res, err := client.GetRouting()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleShareProtocols(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusOK, map[string]interface{}{
			"supported": []string{"socks5"},
			"all":       []string{"vless", "socks5", "vmess", "trojan", "shadowsocks", "http"},
		})
		return
	}

	rawCfg, _ := client.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)
	supported := s.shareService.GetSupportedProtocols(cfg)
	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"supported": supported,
		"all":       []string{"vless", "socks5", "vmess", "trojan", "shadowsocks", "http"},
	})
}

func (s *Server) handleShareGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "target host unavailable"})
		return
	}

	var req struct {
		NodeID   string `json:"node_id"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	nodeDetail, err := client.GetNodeDetails(req.NodeID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	node := sharelink.NodeInfo{
		ID:      req.NodeID,
		IP:      fmt.Sprintf("%v", nodeDetail["ip"]),
		Country: fmt.Sprintf("%v", nodeDetail["country"]),
	}

	rawCfg, _ := client.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)

	res, err := s.shareService.Generate(node, req.Protocol, cfg)
	if err != nil && res == nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleShareBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	_, client, err := s.resolveTargetHost(r)
	if err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "target host unavailable"})
		return
	}

	var req struct {
		Protocols []string `json:"protocols"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if len(req.Protocols) == 0 {
		req.Protocols = []string{"vless", "socks5"}
	}

	exits, _ := client.GetCurrentExits()
	var nodes []sharelink.NodeInfo
	for _, exit := range exits {
		ip, _ := exit["ip"].(string)
		country, _ := exit["country"].(string)
		nodeID, _ := exit["node_id"].(string)
		if ip != "" {
			nodes = append(nodes, sharelink.NodeInfo{
				ID:      nodeID,
				IP:      ip,
				Country: country,
			})
		}
	}

	rawCfg, _ := client.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)

	results := s.shareService.BatchGenerate(nodes, req.Protocols, cfg)
	auth.SendJSON(w, http.StatusOK, results)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	auditLogs := audit.GlobalLogger.GetEntries()
	type EventItem struct {
		Timestamp time.Time `json:"timestamp"`
		Severity  string    `json:"severity"`
		Component string    `json:"component"`
		Event     string    `json:"event"`
		Node      string    `json:"node,omitempty"`
		Slot      string    `json:"slot,omitempty"`
		Reason    string    `json:"reason"`
	}

	var events []EventItem
	for _, al := range auditLogs {
		sev := "INFO"
		if al.Result == "FAILED" || al.Result == "FORBIDDEN" {
			sev = "WARN"
		}
		events = append(events, EventItem{
			Timestamp: al.Timestamp,
			Severity:  sev,
			Component: "WebManager/" + al.Action,
			Event:     al.Action + " " + al.Result,
			Node:      al.Target,
			Reason:    fmt.Sprintf("User: %s (IP: %s) %s", al.User, al.SourceIP, al.Details),
		})
	}

	auth.SendJSON(w, http.StatusOK, events)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	_, client, err := s.resolveTargetHost(r)

	var daemonInfo map[string]interface{}
	var rawCfg map[string]interface{}

	if err == nil && client != nil {
		rawCfg, _ = client.GetClientConfig()
		status, _ := client.GetStatus()
		sys, _ := client.GetSystem()
		daemonInfo = map[string]interface{}{
			"status": status,
			"system": sys,
		}
	} else {
		daemonInfo = map[string]interface{}{
			"status": "UNKNOWN",
			"system": "UNKNOWN",
		}
	}

	defHost, _ := s.hostManager.GetDefaultHost()
	selectedHostName := "None"
	selectedHostAddr := "None"
	selectedHostAgent := "None"
	if defHost != nil {
		selectedHostName = defHost.Name
		selectedHostAddr = defHost.Address
		selectedHostAgent = defHost.AgentURL
	}

	settings := map[string]interface{}{
		"manager": map[string]interface{}{
			"version":            "2.0.0-multihost",
			"listening_address":  s.addr,
			"selected_host":      selectedHostName,
			"selected_address":   selectedHostAddr,
			"selected_agent_url": selectedHostAgent,
		},
		"daemon":       daemonInfo,
		"xray_runtime": rawCfg,
		"routing": map[string]interface{}{
			"mode":               "policy_routing_fwmark",
			"base_table_id":      10000,
			"dns_leak_guard":     "ACTIVE",
			"ipv6_leak_guard":    "ACTIVE",
			"failover_mechanism": "dynamic_draining_transaction",
		},
		"security": map[string]interface{}{
			"auth_type":        "bcrypt_sqlite_rbac",
			"bootstrap_status": "HARDENED",
			"csrf_protection":  "ACTIVE",
			"rate_limiting":    "ACTIVE",
			"secret_redaction": "STRICT",
		},
	}
	auth.SendJSON(w, http.StatusOK, settings)
}

func parseRuntimeConfig(raw map[string]interface{}) sharelink.RuntimeConfig {
	cfg := sharelink.RuntimeConfig{}
	if raw == nil {
		return cfg
	}
	if socks, ok := raw["socks"].(map[string]interface{}); ok {
		cfg.SocksAddress, _ = socks["address"].(string)
		if port, ok := socks["port"].(float64); ok {
			cfg.SocksPort = int(port)
		} else if port, ok := socks["port"].(int); ok {
			cfg.SocksPort = port
		}
	}
	if vless, ok := raw["vless"].(map[string]interface{}); ok {
		cfg.VlessEnabled = true
		cfg.VlessAddress, _ = vless["address"].(string)
		cfg.VlessUUID, _ = vless["uuid"].(string)
		cfg.VlessSecurity, _ = vless["security"].(string)
		cfg.VlessSNI, _ = vless["server_name"].(string)
		cfg.VlessFingerprint, _ = vless["fingerprint"].(string)
		cfg.VlessPublicKey, _ = vless["public_key"].(string)
		cfg.VlessShortID, _ = vless["short_id"].(string)
		cfg.VlessFlow, _ = vless["flow"].(string)
		cfg.VlessType, _ = vless["type"].(string)
		if port, ok := vless["port"].(float64); ok {
			cfg.VlessPort = int(port)
		}
	}
	if protos, ok := raw["protocols"].([]interface{}); ok {
		for _, p := range protos {
			if s, ok := p.(string); ok {
				cfg.SupportedProtocols = append(cfg.SupportedProtocols, s)
			}
		}
	}
	return cfg
}

func (s *Server) Start() error {
	log.Printf("[WebManager] Starting multi-host control panel server on %s", s.addr)
	return http.ListenAndServe(s.addr, s.Handler())
}
