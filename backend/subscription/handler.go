package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/proxy"
	"github.com/NaNA1337/super-proxy-manager/backend/sharelink"
)

type Handler struct {
	store        *Store
	daemonClient *proxy.DaemonClient
	shareService *sharelink.Service
}

func NewHandler(store *Store, client *proxy.DaemonClient, ss *sharelink.Service) *Handler {
	return &Handler{
		store:        store,
		daemonClient: client,
		shareService: ss,
	}
}

type CreateSubRequest struct {
	Name         string              `json:"name"`
	Profile      ProfileType         `json:"profile"`
	Region       string              `json:"region"`
	Protocol     string              `json:"protocol"`
	DurationDays int                 `json:"duration_days"`
}

func (h *Handler) HandleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateSubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.SendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	sess := r.Context().Value(auth.SessionContextKey).(*auth.Session)
	username := "admin"
	if sess != nil {
		username = sess.Username
	}

	if req.Profile == "" {
		req.Profile = ProfileAllActive
	}

	sub, rawToken, err := h.store.Create(req.Name, req.Profile, req.Region, req.Protocol, username, req.DurationDays)
	if err != nil {
		auth.SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Compute public subscription URL
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	subURL := fmt.Sprintf("%s://%s/sub/%s", scheme, r.Host, rawToken)

	resp := CreateSubscriptionResponse{
		Subscription: sub,
		RawToken:     rawToken,
		SubURL:       subURL,
	}

	auth.SendJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	subs := h.store.List()
	auth.SendJSON(w, http.StatusOK, subs)
}

func (h *Handler) HandleRevokeSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid subscription ID", http.StatusBadRequest)
		return
	}
	subID := parts[2]

	if err := h.store.Revoke(subID); err != nil {
		auth.SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	auth.SendJSON(w, http.StatusOK, map[string]string{"status": "revoked", "id": subID})
}

// HandleGetSubscription exports the base64-encoded proxy list for the given token
func (h *Handler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Token required", http.StatusBadRequest)
		return
	}
	rawToken := parts[1]

	sub, err := h.store.GetByToken(rawToken)
	if err != nil {
		// Strictly 403 Forbidden on invalid, revoked or expired token
		http.Error(w, fmt.Sprintf("Subscription rejected: %v", err), http.StatusForbidden)
		return
	}

	// 1. Fetch current exits and candidate nodes
	var candidateNodes []sharelink.NodeInfo
	exits, _ := h.daemonClient.GetCurrentExits()
	for _, exit := range exits {
		ip, _ := exit["ip"].(string)
		country, _ := exit["country"].(string)
		nodeID, _ := exit["node_id"].(string)
		status, _ := exit["status"].(string)
		if ip != "" {
			candidateNodes = append(candidateNodes, sharelink.NodeInfo{
				ID:      nodeID,
				IP:      ip,
				Country: country,
				Status:  status,
			})
		}
	}

	// If profile is all_nodes or region, add qualified nodes
	if sub.Profile == ProfileAllNodes || sub.Profile == ProfileRegion {
		qualified, _ := h.daemonClient.GetPoolQualified()
		for _, q := range qualified {
			ip, _ := q["ip"].(string)
			country, _ := q["country"].(string)
			id, _ := q["id"].(string)
			status, _ := q["status"].(string)
			if ip != "" {
				candidateNodes = append(candidateNodes, sharelink.NodeInfo{
					ID:      id,
					IP:      ip,
					Country: country,
					Status:  status,
				})
			}
		}
	}

	// Fetch runtime config
	rawCfg, _ := h.daemonClient.GetClientConfig()
	cfg := sharelink.RuntimeConfig{}
	if rawCfg != nil {
		if socks, ok := rawCfg["socks"].(map[string]interface{}); ok {
			cfg.SocksAddress, _ = socks["address"].(string)
			if port, ok := socks["port"].(float64); ok {
				cfg.SocksPort = int(port)
			} else if port, ok := socks["port"].(int); ok {
				cfg.SocksPort = port
			}
		}
		if vless, ok := rawCfg["vless"].(map[string]interface{}); ok {
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
		if protos, ok := rawCfg["protocols"].([]interface{}); ok {
			for _, p := range protos {
				if s, ok := p.(string); ok {
					cfg.SupportedProtocols = append(cfg.SupportedProtocols, s)
				}
			}
		}
	}

	var supportedProtocols []string
	if sub.ProtocolFilter != "" {
		supportedProtocols = []string{sub.ProtocolFilter}
	} else {
		supportedProtocols = h.shareService.GetSupportedProtocols(cfg)
		if len(supportedProtocols) == 0 {
			supportedProtocols = []string{"socks5"}
		}
	}

	var uris []string
	seen := make(map[string]bool)

	for _, node := range candidateNodes {
		// Region filter check
		if sub.Profile == ProfileRegion && sub.RegionFilter != "" {
			if !strings.EqualFold(node.Country, sub.RegionFilter) {
				continue
			}
		}

		for _, proto := range supportedProtocols {
			res, err := h.shareService.Generate(node, proto, cfg)
			if err == nil && res != nil && res.URI != "" {
				if !seen[res.URI] {
					seen[res.URI] = true
					uris = append(uris, res.URI)
				}
			}
		}
	}

	rawContent := strings.Join(uris, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(rawContent))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=107374182400; expire=0")
	w.Header().Set("Profile-Update-Interval", "24")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(encoded))
}

// RedactedLoggingMiddleware ensures that /sub/<token> is never logged with the raw secret token
func RedactedLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := r.RequestURI
		if strings.HasPrefix(uri, "/sub/") {
			parts := strings.Split(uri, "/")
			if len(parts) >= 3 {
				// Replace raw token with [REDACTED] in logs
				log.Printf("[Access-Redacted] %s %s from %s", r.Method, "/sub/[REDACTED]", r.RemoteAddr)
			}
		} else {
			log.Printf("[Access] %s %s from %s", r.Method, uri, r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}
