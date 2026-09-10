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

type ClientProviderFunc func(hostID string) (*proxy.DaemonClient, error)

type Handler struct {
	store          *Store
	clientProvider ClientProviderFunc
	legacyClient   *proxy.DaemonClient
	shareService   *sharelink.Service
}

func NewHandlerWithProvider(store *Store, provider ClientProviderFunc, ss *sharelink.Service) *Handler {
	return &Handler{
		store:          store,
		clientProvider: provider,
		shareService:   ss,
	}
}

func NewHandler(store *Store, client *proxy.DaemonClient, ss *sharelink.Service) *Handler {
	return &Handler{
		store:        store,
		legacyClient: client,
		shareService: ss,
	}
}

func (h *Handler) getClient(hostID string) (*proxy.DaemonClient, error) {
	if h.clientProvider != nil {
		return h.clientProvider(hostID)
	}
	if h.legacyClient != nil {
		return h.legacyClient, nil
	}
	return nil, fmt.Errorf("no client available")
}

type CreateSubRequest struct {
	Name         string      `json:"name"`
	HostID       string      `json:"host_id"`
	Profile      ProfileType `json:"profile"`
	Region       string      `json:"region"`
	Protocol     string      `json:"protocol"`
	DurationDays int         `json:"duration_days"`
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

	sub, rawToken, err := h.store.Create(req.Name, req.HostID, req.Profile, req.Region, req.Protocol, username, req.DurationDays)
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

	client, err := h.getClient(sub.HostID)
	if err != nil {
		http.Error(w, "Target host client unavailable", http.StatusServiceUnavailable)
		return
	}

	// 1. Attempt canonical profiles first directly from Agent
	var links []string
	allCfg, cfgErr := client.GetAllClientConfig("")
	if cfgErr == nil && allCfg != nil && allCfg.Available && len(allCfg.Nodes) > 0 {
		for _, n := range allCfg.Nodes {
			if sub.Profile == ProfileRegion && sub.RegionFilter != "" {
				if !strings.EqualFold(n.Country, sub.RegionFilter) {
					continue
				}
			}
			for _, p := range n.Profiles {
				if p.Format == "uri" || p.ID == "vless" {
					links = append(links, p.Content)
					break // One primary URI per node
				}
			}
		}
	}

	// 2. If canonical profiles were not returned, fall back to discovered candidate nodes
	if len(links) == 0 {
		var candidateNodes []sharelink.NodeInfo
		exits, _ := client.GetCurrentExits()
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

		// Fallback to pool qualified if no active exits or profile is AllNodes
		if len(candidateNodes) == 0 || sub.Profile == ProfileAllNodes {
			qualified, _ := client.GetPoolQualified()
			for _, q := range qualified {
				ip, _ := q["ip"].(string)
				country, _ := q["country"].(string)
				nodeID, _ := q["id"].(string)
				if ip != "" {
					candidateNodes = append(candidateNodes, sharelink.NodeInfo{
						ID:      nodeID,
						IP:      ip,
						Country: country,
					})
				}
			}
		}

		rawCfg, _ := client.GetClientConfig()
		cfg := parseRuntimeConfig(rawCfg)

		for _, node := range candidateNodes {
			if sub.Profile == ProfileRegion && sub.RegionFilter != "" {
				if !strings.EqualFold(node.Country, sub.RegionFilter) {
					continue
				}
			}

			targetProtocol := "vless"
			if sub.Profile == ProfileProtocol && sub.ProtocolFilter != "" {
				targetProtocol = strings.ToLower(sub.ProtocolFilter)
			}

			res, err := h.shareService.Generate(node, targetProtocol, cfg)
			if err == nil && res != nil && res.Supported && res.URI != "" {
				links = append(links, res.URI)
			}
		}
	}

	if len(links) == 0 {
		// Provide informative header if no nodes match criteria
		links = append(links, "# No active nodes matching criteria")
	}

	// 4. Encode as Base64 proxy list
	joined := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(joined))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=107374182400; expire=0")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(encoded))
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

func RedactedLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/sub/") {
			parts := strings.Split(strings.Trim(path, "/"), "/")
			if len(parts) >= 2 && len(parts[1]) > 0 {
				path = "/sub/[REDACTED]"
			}
		}
		log.Printf("[HTTP] %s %s from %s", r.Method, path, auth.GetClientIP(r))
		next.ServeHTTP(w, r)
	})
}
