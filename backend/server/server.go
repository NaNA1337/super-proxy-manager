package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/audit"
	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/proxy"
	"github.com/NaNA1337/super-proxy-manager/backend/sharelink"
	"github.com/NaNA1337/super-proxy-manager/backend/subscription"
)

type Server struct {
	addr         string
	daemonClient *proxy.DaemonClient
	shareService *sharelink.Service
	subStore     *subscription.Store
	subHandler   *subscription.Handler
	distFS       fs.FS
	mux          *http.ServeMux
}

func NewServer(addr string, distFS fs.FS) *Server {
	if addr == "" {
		addr = ":8443"
	}

	client := proxy.NewDaemonClient()
	ss := sharelink.NewService()
	subStore := subscription.NewStore()
	subHandler := subscription.NewHandler(subStore, client, ss)

	s := &Server{
		addr:         addr,
		daemonClient: client,
		shareService: ss,
		subStore:     subStore,
		subHandler:   subHandler,
		distFS:       distFS,
		mux:          http.NewServeMux(),
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

	// 3. Daemon Control Plane (Authenticated)
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

	// Slot switch (Restricted to admin + CSRF enforced)
	s.mux.HandleFunc("/api/daemon/slots/", s.requireAdminHandler(s.handleDaemonSlotAction))

	// 4. ShareLink Endpoints
	s.mux.HandleFunc("/api/sharelinks/protocols", s.requireAuthHandler(s.handleShareProtocols))
	s.mux.HandleFunc("/api/sharelinks/generate", s.requireAuthHandler(s.handleShareGenerate))
	s.mux.HandleFunc("/api/sharelinks/batch", s.requireAuthHandler(s.handleShareBatch))

	// 5. Subscription Management
	s.mux.HandleFunc("/api/subscriptions", s.requireAuthHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Require admin for creating subscriptions
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

	// 6. Audit & Events
	s.mux.HandleFunc("/api/audit", s.requireAuthHandler(audit.HandleGetAuditLogs))
	s.mux.HandleFunc("/api/events", s.requireAuthHandler(s.handleEvents))

	// 7. Settings (Read-only)
	s.mux.HandleFunc("/api/settings", s.requireAuthHandler(s.handleSettings))

	// 8. Static SPA files or fallback
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
			auth.SessionMiddleware(s.mux),
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

	user, err := auth.Authenticate(req.Username, req.Password)
	if err != nil {
		audit.GlobalLogger.Log(req.Username, "unknown", "login", "auth", "FAILED", ip, "invalid credentials")
		auth.SendJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	sess, err := auth.CreateSession(user, 24*time.Hour)
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	auth.SetSessionCookie(w, sess.Token, sess.ExpiresAt, isSecure)

	audit.GlobalLogger.Log(user.Username, string(user.Role), "login", "auth", "SUCCESS", ip, "session created")

	auth.SendJSON(w, http.StatusOK, map[string]interface{}{
		"username":   user.Username,
		"role":       user.Role,
		"token":      sess.Token,
		"csrf_token": sess.CSRFToken,
		"expires_at": sess.ExpiresAt,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := r.Context().Value(auth.SessionContextKey).(*auth.Session); ok && sess != nil {
		auth.RevokeSession(sess.Token)
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
		"authenticated": true,
		"username":      sess.Username,
		"role":          sess.Role,
		"csrf_token":    sess.CSRFToken,
		"expires_at":    sess.ExpiresAt,
	})
}

func (s *Server) handleDaemonStatus(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetStatus()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSystem(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetSystem()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonCurrentExits(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetCurrentExits()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSlots(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetSlots()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonPool(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetPool()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonPoolQualified(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetPoolQualified()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonNodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	country := q.Get("country")
	status := q.Get("status")
	search := q.Get("search")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	res, err := s.daemonClient.GetNodes(country, status, search, limit, offset)
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonNodeDetails(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}
	nodeID := parts[3]
	res, err := s.daemonClient.GetNodeDetails(nodeID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonSlotAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Expected: /api/daemon/slots/{slot}/switch
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
	res, code, err := s.daemonClient.SwitchSlot(slot, req.NodeID)
	if err != nil {
		audit.GlobalLogger.Log(sess.Username, string(sess.Role), "switch_slot", fmt.Sprintf("slot-%d -> %s", slot, req.NodeID), "FAILED", auth.GetClientIP(r), err.Error())
		auth.SendJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	audit.GlobalLogger.Log(sess.Username, string(sess.Role), "switch_slot", fmt.Sprintf("slot-%d -> %s", slot, req.NodeID), "SUCCESS", auth.GetClientIP(r), "")
	auth.SendJSON(w, code, res)
}

func (s *Server) handleDaemonOperation(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "Operation ID required", http.StatusBadRequest)
		return
	}
	opID := parts[3]
	res, err := s.daemonClient.GetOperation(opID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleDaemonMetrics(w http.ResponseWriter, r *http.Request) {
	text, err := s.daemonClient.GetMetrics()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

func (s *Server) handleDaemonRouting(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemonClient.GetRouting()
	if err != nil {
		auth.SendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	auth.SendJSON(w, http.StatusOK, res)
}

func (s *Server) handleShareProtocols(w http.ResponseWriter, r *http.Request) {
	rawCfg, _ := s.daemonClient.GetClientConfig()
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

	var req struct {
		NodeID   string `json:"node_id"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	nodeDetail, err := s.daemonClient.GetNodeDetails(req.NodeID)
	if err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	node := sharelink.NodeInfo{
		ID:      req.NodeID,
		IP:      fmt.Sprintf("%v", nodeDetail["ip"]),
		Country: fmt.Sprintf("%v", nodeDetail["country"]),
	}

	rawCfg, _ := s.daemonClient.GetClientConfig()
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

	var req struct {
		Protocols []string `json:"protocols"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if len(req.Protocols) == 0 {
		req.Protocols = []string{"vless", "socks5"}
	}

	exits, _ := s.daemonClient.GetCurrentExits()
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

	rawCfg, _ := s.daemonClient.GetClientConfig()
	cfg := parseRuntimeConfig(rawCfg)

	results := s.shareService.BatchGenerate(nodes, req.Protocols, cfg)
	auth.SendJSON(w, http.StatusOK, results)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Synthesize events from audit logs and recent slot activities
	auditLogs := audit.GlobalLogger.GetEntries()
	type EventItem struct {
		Timestamp time.Time `json:"timestamp"`
		Severity  string    `json:"severity"` // INFO, WARN, ERROR
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
	// Provide clean, read-only configuration view
	rawCfg, _ := s.daemonClient.GetClientConfig()
	status, _ := s.daemonClient.GetStatus()
	sys, _ := s.daemonClient.GetSystem()

	settings := map[string]interface{}{
		"daemon": map[string]interface{}{
			"status":  status,
			"system":  sys,
			"api_url": os.Getenv("AGENT_API_URL"),
		},
		"xray_runtime": rawCfg,
		"routing": map[string]interface{}{
			"mode":               "policy_routing_fwmark",
			"base_table_id":      10000,
			"dns_leak_guard":     "ACTIVE",
			"ipv6_leak_guard":    "ACTIVE",
			"failover_mechanism": "dynamic_draining_transaction",
		},
		"security": map[string]interface{}{
			"auth_type":         "bcrypt_session_rbac",
			"csrf_protection":   "ACTIVE",
			"rate_limiting":     "ACTIVE",
			"secret_redaction":  "STRICT",
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
	log.Printf("[WebManager] Starting control panel server on %s", s.addr)
	return http.ListenAndServe(s.addr, s.Handler())
}
