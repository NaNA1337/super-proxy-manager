package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
	"github.com/NaNA1337/super-proxy-manager/backend/proxy"
)

func TestE2E_FullFlow(t *testing.T) {
	_ = os.Setenv("ALLOW_PRIVATE_HOSTS", "true")
	defer os.Unsetenv("ALLOW_PRIVATE_HOSTS")

	// 1. Setup Fresh Test SQLite Database
	tempDir, err := os.MkdirTemp("", "spm_e2e_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	database, err := db.InitDB(tempDir)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer database.Close()

	// 2. Bootstrap Check - First boot generates random temporary password
	bootstrapped, tempPass, err := auth.CheckOrInitBootstrap(database, "127.0.0.1:8080")
	if err != nil || !bootstrapped || tempPass == "" {
		t.Fatalf("bootstrap failed: bootstrapped=%v, tempPass=%s, err=%v", bootstrapped, tempPass, err)
	}

	distFS, _ := embedded.GetDistFS()
	srv := NewServer("127.0.0.1:0", database, distFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 3. Test Static SPA Loading
	resp, err := client.Get(ts.URL + "/")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for root SPA, got %d", resp.StatusCode)
	}

	// 4. Failed Login Attempt with invalid password
	badLoginBody := []byte(`{"username":"admin","password":"wrong-password-12345"}`)
	resp, err = client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(badLoginBody))
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong password, got %d", resp.StatusCode)
	}

	// 5. Successful Login using Temporary Bootstrap Password
	goodLoginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": tempPass,
	})
	resp, err = client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(goodLoginBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid admin bootstrap login, got %d", resp.StatusCode)
	}

	var tempLoginRes struct {
		MustChangePassword bool   `json:"must_change_password"`
		Username           string `json:"username"`
		Role               string `json:"role"`
		Token              string `json:"token"`
		CSRFToken          string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tempLoginRes); err != nil {
		t.Fatalf("failed to decode bootstrap login response: %v", err)
	}
	if !tempLoginRes.MustChangePassword {
		t.Fatalf("expected MustChangePassword=true on bootstrap login")
	}

	// Extract session cookie
	var tempCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			tempCookie = c
			break
		}
	}
	if tempCookie == nil {
		t.Fatalf("missing session cookie")
	}

	// 6. Verify Normal API is BLOCKED with 403 (password_change_required)
	hostsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/hosts", nil)
	hostsReq.AddCookie(tempCookie)
	hostsResp, err := client.Do(hostsReq)
	if err != nil || hostsResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on /api/hosts when password change is required, got %d", hostsResp.StatusCode)
	}

	// 7. Perform Forced Password Change
	newPassword := "NewAdminPassword2026#!"
	changePassBody, _ := json.Marshal(map[string]string{
		"current_password": tempPass,
		"new_password":     newPassword,
		"confirm_password": newPassword,
	})
	changeReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/change-password", bytes.NewReader(changePassBody))
	changeReq.Header.Set("Content-Type", "application/json")
	changeReq.Header.Set("X-CSRF-Token", tempLoginRes.CSRFToken)
	changeReq.AddCookie(tempCookie)

	changeResp, err := client.Do(changeReq)
	if err != nil || changeResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to change password: status %d", changeResp.StatusCode)
	}

	// Extract new session cookie after password change
	var newSessionCookie *http.Cookie
	for _, c := range changeResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			newSessionCookie = c
			break
		}
	}
	if newSessionCookie == nil {
		t.Fatalf("expected new session cookie after password change")
	}

	var changeData struct {
		MustChangePassword bool   `json:"must_change_password"`
		CSRFToken          string `json:"csrf_token"`
	}
	_ = json.NewDecoder(changeResp.Body).Decode(&changeData)
	if changeData.MustChangePassword {
		t.Fatalf("expected must_change_password=false after change")
	}

	// Helper for authenticated requests with new session
	authReq := func(method, path string, body []byte) *http.Request {
		var r *http.Request
		if body != nil {
			r, _ = http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		r.AddCookie(newSessionCookie)
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete {
			r.Header.Set("X-CSRF-Token", changeData.CSRFToken)
		}
		return r
	}

	// 8. Verify /api/auth/me reflects active authenticated user
	meResp, err := client.Do(authReq(http.MethodGet, "/api/auth/me", nil))
	if err != nil || meResp.StatusCode != http.StatusOK {
		t.Fatalf("failed /api/auth/me after password change: status %d", meResp.StatusCode)
	}

	// 9. Mock Daemon Setup (simulating real super-proxy daemon)
	daemonToken := "tokyo-daemon-secret-token-12345"
	mockDaemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+daemonToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version":   "1.5.0",
				"server_id": "tokyo-01",
				"uptime":    3600,
			})
		case "/api/v1/system":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"CPU": 5.5, "Memory": 20.0, "Load": []float64{0.1, 0.2, 0.3},
			})
		case "/api/v1/current-exits":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"node_id": "node-jp-01",
					"ip":      "203.0.113.10",
					"country": "JP",
					"status":  "ACTIVE",
				},
			})
		case "/api/v1/client-config":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"protocols": []string{"vless", "socks5"},
				"socks": map[string]interface{}{
					"address": "127.0.0.1",
					"port":    1080,
				},
				"vless": map[string]interface{}{
					"address":     "jp01.example.com",
					"port":        443,
					"uuid":        "e96f131a-5b12-4d56-b072-4b7b2ce89012",
					"security":    "reality",
					"server_name": "gateway.icloud.com",
					"fingerprint": "chrome",
					"public_key":  "testPublicKeyBase64StringForRealityVerification123",
					"short_id":    "abcd1234ef",
					"flow":        "xtls-rprx-vision",
					"type":        "tcp",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockDaemon.Close()

	// 10. Test Pre-Save Host Connection
	testHostBody, _ := json.Marshal(map[string]string{
		"agent_url": mockDaemon.URL,
		"token":     daemonToken,
	})
	testHostResp, err := client.Do(authReq(http.MethodPost, "/api/hosts/test", testHostBody))
	if err != nil || testHostResp.StatusCode != http.StatusOK {
		t.Fatalf("failed /api/hosts/test: status %d", testHostResp.StatusCode)
	}

	// 11. Add Host
	addHostBody, _ := json.Marshal(map[string]interface{}{
		"name":       "Tokyo-01",
		"address":    "jp01.example.com",
		"agent_url":  mockDaemon.URL,
		"token":      daemonToken,
		"is_default": true,
	})
	addHostResp, err := client.Do(authReq(http.MethodPost, "/api/hosts", addHostBody))
	if err != nil || addHostResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to add host: status %d", addHostResp.StatusCode)
	}

	var createdHost struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.NewDecoder(addHostResp.Body).Decode(&createdHost)
	if createdHost.ID == "" || createdHost.Name != "Tokyo-01" {
		t.Fatalf("invalid created host: %+v", createdHost)
	}

	// 12. Verify List Hosts does NOT leak daemon token
	listHostsResp, err := client.Do(authReq(http.MethodGet, "/api/hosts", nil))
	if err != nil || listHostsResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to list hosts: status %d", listHostsResp.StatusCode)
	}
	listHostsBytes := new(bytes.Buffer)
	_, _ = listHostsBytes.ReadFrom(listHostsResp.Body)
	if strings.Contains(listHostsBytes.String(), daemonToken) {
		t.Fatalf("SECURITY VIOLATION: Daemon token exposed in /api/hosts JSON: %s", listHostsBytes.String())
	}

	// 13. Test Host-Aware Client Links
	linksReq := authReq(http.MethodGet, "/api/hosts/"+createdHost.ID+"/client-links", nil)
	linksResp, err := client.Do(linksReq)
	if err != nil || linksResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get client links: status %d", linksResp.StatusCode)
	}

	// 14. Test Structured Export Plain Text
	allTextReq := authReq(http.MethodGet, "/api/hosts/"+createdHost.ID+"/client-links/all", nil)
	allTextResp, err := client.Do(allTextReq)
	if err != nil || allTextResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get client links all text: status %d", allTextResp.StatusCode)
	}
	allTextBuf := new(bytes.Buffer)
	_, _ = allTextBuf.ReadFrom(allTextResp.Body)
	allText := allTextBuf.String()
	if !strings.Contains(allText, "SUPER-PROXY CLIENT LINKS EXPORT") {
		t.Errorf("expected structured export header in text output: %s", allText)
	}

	// 15. Test Create Subscription bound to Host
	subReqBody, _ := json.Marshal(map[string]interface{}{
		"name":          "E2E Host Sub",
		"host_id":       createdHost.ID,
		"profile":       "active_only",
		"duration_days": 30,
	})
	createSubResp, err := client.Do(authReq(http.MethodPost, "/api/subscriptions", subReqBody))
	if err != nil || createSubResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create subscription: status %d", createSubResp.StatusCode)
	}

	var subRes struct {
		Subscription struct {
			ID string `json:"id"`
		} `json:"subscription"`
		RawToken string `json:"raw_token"`
	}
	_ = json.NewDecoder(createSubResp.Body).Decode(&subRes)

	// 16. Access Public Subscription Endpoint
	subURL := ts.URL + "/sub/" + subRes.RawToken
	subResp, err := client.Get(subURL)
	if err != nil || subResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch subscription: status %d", subResp.StatusCode)
	}
	subBytes := new(bytes.Buffer)
	_, _ = subBytes.ReadFrom(subResp.Body)
	_, err = base64.StdEncoding.DecodeString(subBytes.String())
	if err != nil {
		t.Fatalf("subscription payload is not valid base64: %v", err)
	}

	// 17. Revoke Subscription
	revokeResp, err := client.Do(authReq(http.MethodPost, "/api/subscriptions/"+subRes.Subscription.ID+"/revoke", nil))
	if err != nil || revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to revoke subscription: status %d", revokeResp.StatusCode)
	}

	// 18. Subsequent access rejected (403)
	subRejectedResp, err := client.Get(subURL)
	if err != nil || subRejectedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on revoked sub token, got %d", subRejectedResp.StatusCode)
	}

	// 19. Audit Logs Check - Verify zero token/password leakage
	auditResp, err := client.Do(authReq(http.MethodGet, "/api/audit", nil))
	if err != nil || auditResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch audit logs: status %d", auditResp.StatusCode)
	}
	auditBuf := new(bytes.Buffer)
	_, _ = auditBuf.ReadFrom(auditResp.Body)
	auditStr := auditBuf.String()
	if strings.Contains(auditStr, tempPass) || strings.Contains(auditStr, newPassword) || strings.Contains(auditStr, daemonToken) || strings.Contains(auditStr, subRes.RawToken) {
		t.Fatalf("SECURITY VIOLATION: Secret leaked in audit logs: %s", auditStr)
	}
}

func TestE2E_MultiHostIsolationAndCanonicalClientConfig(t *testing.T) {
	_ = os.Setenv("ALLOW_PRIVATE_HOSTS", "true")
	defer os.Unsetenv("ALLOW_PRIVATE_HOSTS")

	tempDir, err := os.MkdirTemp("", "spm_e2e_isolation_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	database, err := db.InitDB(tempDir)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer database.Close()

	_, tempPass, _ := auth.CheckOrInitBootstrap(database, "127.0.0.1:8080")

	distFS, _ := embedded.GetDistFS()
	srv := NewServer("127.0.0.1:0", database, distFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. Login & Change Password
	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": tempPass})
	loginResp, err := client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil || loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %v", err)
	}
	var loginData struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.NewDecoder(loginResp.Body).Decode(&loginData)

	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}

	newPassword := "Production@Secure2026!#"
	chgPassBody, _ := json.Marshal(map[string]string{
		"current_password": tempPass,
		"new_password":     newPassword,
		"confirm_password": newPassword,
	})
	chgReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/change-password", bytes.NewReader(chgPassBody))
	chgReq.Header.Set("Content-Type", "application/json")
	chgReq.Header.Set("X-CSRF-Token", loginData.CSRFToken)
	chgReq.AddCookie(sessionCookie)
	chgResp, err := client.Do(chgReq)
	if err != nil || chgResp.StatusCode != http.StatusOK {
		t.Fatalf("change password failed: status %d, err %v", chgResp.StatusCode, err)
	}
	for _, c := range chgResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
		}
	}
	var chgData struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.NewDecoder(chgResp.Body).Decode(&chgData)
	if chgData.CSRFToken != "" {
		loginData.CSRFToken = chgData.CSRFToken
	}

	authReq := func(method, path string, body []byte) *http.Request {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, _ := http.NewRequest(method, ts.URL+path, bodyReader)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("X-CSRF-Token", loginData.CSRFToken)
		req.AddCookie(sessionCookie)
		return req
	}

	// 2. Setup Agent A (Tokyo)
	agentAToken := "agent-token-tokyo-7788"
	agentA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+agentAToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version":   "1.5.0",
				"server_id": "tokyo-agent-01",
				"region":    "JP",
			})
		case "/api/v1/client-config/all":
			_ = json.NewEncoder(w).Encode(proxy.AllClientConfigResponse{
				Available: true,
				Nodes: []proxy.NodeClientConfig{
					{
						Available:       true,
						NodeID:          "node-tokyo-alpha",
						NodeIP:          "198.51.100.1",
						Country:         "JP",
						EndpointAddress: "tokyo.example.com",
						EndpointPort:    443,
						Protocol:        "vless",
						Profiles: []proxy.CanonicalProfile{
							{
								ID:       "vless",
								Name:     "VLESS URI",
								Format:   "uri",
								Filename: "vless.txt",
								Content:  "vless://tokyo-uuid@tokyo.example.com:443?security=reality#Tokyo-Alpha",
								CanQR:    true,
							},
							{
								ID:       "clash",
								Name:     "Clash Meta",
								Format:   "yaml",
								Filename: "clash-meta.yaml",
								Content:  "proxies:\n  - name: Tokyo-Alpha\n    type: vless",
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer agentA.Close()

	// 3. Setup Agent B (Seoul)
	agentBToken := "agent-token-seoul-9900"
	agentB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+agentBToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version":   "1.5.0",
				"server_id": "seoul-agent-01",
				"region":    "KR",
			})
		case "/api/v1/client-config/all":
			_ = json.NewEncoder(w).Encode(proxy.AllClientConfigResponse{
				Available: true,
				Nodes: []proxy.NodeClientConfig{
					{
						Available:       true,
						NodeID:          "node-seoul-beta",
						NodeIP:          "203.0.113.2",
						Country:         "KR",
						EndpointAddress: "seoul.example.com",
						EndpointPort:    443,
						Protocol:        "vless",
						Profiles: []proxy.CanonicalProfile{
							{
								ID:       "vless",
								Name:     "VLESS URI",
								Format:   "uri",
								Filename: "vless.txt",
								Content:  "vless://seoul-uuid@seoul.example.com:443?security=reality#Seoul-Beta",
								CanQR:    true,
							},
							{
								ID:       "sing-box",
								Name:     "sing-box",
								Format:   "json",
								Filename: "sing-box.json",
								Content:  `{"type":"vless","server":"seoul.example.com"}`,
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer agentB.Close()

	// 4. Add Tokyo Host
	addTokyoBody, _ := json.Marshal(map[string]interface{}{
		"name":       "Tokyo",
		"address":    "jp.superproxy.io",
		"agent_url":  agentA.URL,
		"token":      agentAToken,
		"is_default": true,
	})
	tokyoResp, err := client.Do(authReq(http.MethodPost, "/api/hosts", addTokyoBody))
	if err != nil || tokyoResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to add Tokyo host: %v", err)
	}
	var tokyoHost struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(tokyoResp.Body).Decode(&tokyoHost)

	// 5. Add Seoul Host
	addSeoulBody, _ := json.Marshal(map[string]interface{}{
		"name":       "Seoul",
		"address":    "kr.superproxy.io",
		"agent_url":  agentB.URL,
		"token":      agentBToken,
		"is_default": false,
	})
	seoulResp, err := client.Do(authReq(http.MethodPost, "/api/hosts", addSeoulBody))
	if err != nil || seoulResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to add Seoul host: %v", err)
	}
	var seoulHost struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(seoulResp.Body).Decode(&seoulHost)

	// 6. MULTI-HOST ISOLATION TEST
	// Query Tokyo Client Config
	cfgTokyoResp, err := client.Do(authReq(http.MethodGet, "/api/hosts/"+tokyoHost.ID+"/client-config", nil))
	if err != nil || cfgTokyoResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get Tokyo client config: status %d", cfgTokyoResp.StatusCode)
	}
	var tokyoAllCfg proxy.AllClientConfigResponse
	_ = json.NewDecoder(cfgTokyoResp.Body).Decode(&tokyoAllCfg)

	if len(tokyoAllCfg.Nodes) != 1 || tokyoAllCfg.Nodes[0].NodeID != "node-tokyo-alpha" {
		t.Fatalf("isolation failure: Tokyo should only have node-tokyo-alpha, got: %+v", tokyoAllCfg.Nodes)
	}
	// Verify Seoul data did NOT leak into Tokyo
	for _, n := range tokyoAllCfg.Nodes {
		if strings.Contains(n.NodeID, "seoul") {
			t.Fatalf("CRITICAL SECURITY LEAK: Seoul node found in Tokyo response!")
		}
	}

	// Query Seoul Client Config
	cfgSeoulResp, err := client.Do(authReq(http.MethodGet, "/api/hosts/"+seoulHost.ID+"/client-config", nil))
	if err != nil || cfgSeoulResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get Seoul client config: status %d", cfgSeoulResp.StatusCode)
	}
	var seoulAllCfg proxy.AllClientConfigResponse
	_ = json.NewDecoder(cfgSeoulResp.Body).Decode(&seoulAllCfg)

	if len(seoulAllCfg.Nodes) != 1 || seoulAllCfg.Nodes[0].NodeID != "node-seoul-beta" {
		t.Fatalf("isolation failure: Seoul should only have node-seoul-beta, got: %+v", seoulAllCfg.Nodes)
	}
	// Verify Tokyo data did NOT leak into Seoul
	for _, n := range seoulAllCfg.Nodes {
		if strings.Contains(n.NodeID, "tokyo") {
			t.Fatalf("CRITICAL SECURITY LEAK: Tokyo node found in Seoul response!")
		}
	}

	// 7. Test Specific Node Client Config Route
	nodeReq := authReq(http.MethodGet, "/api/hosts/"+tokyoHost.ID+"/nodes/node-tokyo-alpha/client-config", nil)
	nodeResp, err := client.Do(nodeReq)
	if err != nil || nodeResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get node-specific client config: status %d", nodeResp.StatusCode)
	}
	var nodeConfig proxy.AllClientConfigResponse
	_ = json.NewDecoder(nodeResp.Body).Decode(&nodeConfig)
	if len(nodeConfig.Nodes) == 0 || len(nodeConfig.Nodes[0].Profiles) < 2 {
		t.Fatalf("expected node profiles to be returned: %+v", nodeConfig)
	}

	// 8. BATCH EXPORT ZIP TEST
	exportBody, _ := json.Marshal(map[string]interface{}{
		"selections": []map[string]string{
			{"host_id": tokyoHost.ID, "node_id": "node-tokyo-alpha"},
			{"host_id": seoulHost.ID, "node_id": "node-seoul-beta"},
		},
	})
	exportReq := authReq(http.MethodPost, "/api/client-configs/export-zip", exportBody)
	exportResp, err := client.Do(exportReq)
	if err != nil || exportResp.StatusCode != http.StatusOK {
		t.Fatalf("failed export-zip: status %d", exportResp.StatusCode)
	}
	if exportResp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("expected application/zip Content-Type, got %s", exportResp.Header.Get("Content-Type"))
	}

	zipBytes, err := io.ReadAll(exportResp.Body)
	if err != nil {
		t.Fatalf("failed to read zip bytes: %v", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("invalid zip archive returned: %v", err)
	}

	foundTokyoFile := false
	foundSeoulFile := false
	for _, f := range zipReader.File {
		if strings.Contains(f.Name, "Tokyo/node-tokyo-alpha/vless.txt") {
			foundTokyoFile = true
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(content), "vless://tokyo-uuid") {
				t.Errorf("expected Tokyo vless content in zip, got: %s", string(content))
			}
		}
		if strings.Contains(f.Name, "Seoul/node-seoul-beta/sing-box.json") {
			foundSeoulFile = true
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(content), "seoul.example.com") {
				t.Errorf("expected Seoul sing-box content in zip, got: %s", string(content))
			}
		}
	}
	if !foundTokyoFile {
		t.Errorf("Tokyo file missing in generated ZIP")
	}
	if !foundSeoulFile {
		t.Errorf("Seoul file missing in generated ZIP")
	}

	// 9. CLIENT CONFIG AUDIT TEST (Zero secret leakage)
	auditEventBody, _ := json.Marshal(map[string]string{
		"host_id":    tokyoHost.ID,
		"node_id":    "node-tokyo-alpha",
		"action":     "copy",
		"profile_id": "vless",
	})
	auditEventReq := authReq(http.MethodPost, "/api/client-configs/audit", auditEventBody)
	auditEventResp, err := client.Do(auditEventReq)
	if err != nil || auditEventResp.StatusCode != http.StatusOK {
		t.Fatalf("failed client-configs audit: status %d", auditEventResp.StatusCode)
	}

	// Verify Audit Log
	auditListReq := authReq(http.MethodGet, "/api/audit", nil)
	auditListResp, err := client.Do(auditListReq)
	if err != nil || auditListResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch audit log: status %d", auditListResp.StatusCode)
	}
	auditContent, _ := io.ReadAll(auditListResp.Body)
	auditStr := string(auditContent)

	if !strings.Contains(auditStr, "copy_client_config") {
		t.Errorf("expected copy_client_config in audit log: %s", auditStr)
	}
	if strings.Contains(auditStr, "tokyo-uuid") || strings.Contains(auditStr, "agent-token") {
		t.Fatalf("SECURITY VIOLATION: UUID or agent token leaked in audit log: %s", auditStr)
	}
}

func TestE2E_StaleDataInvalidation(t *testing.T) {
	_ = os.Setenv("ALLOW_PRIVATE_HOSTS", "true")
	defer os.Unsetenv("ALLOW_PRIVATE_HOSTS")

	tempDir, err := os.MkdirTemp("", "spm_e2e_stale_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	database, err := db.InitDB(tempDir)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer database.Close()

	_, tempPass, _ := auth.CheckOrInitBootstrap(database, "127.0.0.1:8080")

	distFS, _ := embedded.GetDistFS()
	srv := NewServer("127.0.0.1:0", database, distFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// Login & Change Password
	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": tempPass})
	loginResp, _ := client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	var loginData struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.NewDecoder(loginResp.Body).Decode(&loginData)
	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
		}
	}

	chgPassBody, _ := json.Marshal(map[string]string{
		"current_password": tempPass,
		"new_password":     "Production@Secure2026!#",
		"confirm_password": "Production@Secure2026!#",
	})
	chgReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/change-password", bytes.NewReader(chgPassBody))
	chgReq.Header.Set("Content-Type", "application/json")
	chgReq.Header.Set("X-CSRF-Token", loginData.CSRFToken)
	chgReq.AddCookie(sessionCookie)
	chgResp, err := client.Do(chgReq)
	if err != nil || chgResp.StatusCode != http.StatusOK {
		t.Fatalf("change password failed: status %d", chgResp.StatusCode)
	}
	for _, c := range chgResp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
		}
	}
	var chgData struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.NewDecoder(chgResp.Body).Decode(&chgData)
	if chgData.CSRFToken != "" {
		loginData.CSRFToken = chgData.CSRFToken
	}

	authReq := func(method, path string) *http.Request {
		req, _ := http.NewRequest(method, ts.URL+path, nil)
		req.Header.Set("X-CSRF-Token", loginData.CSRFToken)
		req.AddCookie(sessionCookie)
		return req
	}

	// Mock Daemon with dynamic Xray runtime availability toggle
	var xrayOnline int32 = 1

	mockDaemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy", "version": "1.0"})
		case "/api/v1/client-config/all":
			if atomic.LoadInt32(&xrayOnline) == 0 {
				// Xray is stopped / unavailable
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "runtime endpoint unavailable: Xray process stopped",
				})
				return
			}

			// Active runtime
			_ = json.NewEncoder(w).Encode(proxy.AllClientConfigResponse{
				Available: true,
				Nodes: []proxy.NodeClientConfig{
					{
						Available: true,
						NodeID:    "node-dyn-01",
						Profiles: []proxy.CanonicalProfile{
							{
								ID:      "vless",
								Format:  "uri",
								Content: "vless://active-test-uuid@example.com:443",
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockDaemon.Close()

	// Register host
	h, err := srv.hostManager.CreateHost("DynamicHost", "dyn.example.com", mockDaemon.URL, "token", true)
	if err != nil {
		t.Fatalf("failed to create dynamic host: %v", err)
	}

	// 1. Initial request -> must return active profiles (and cache them)
	res1, err := client.Do(authReq(http.MethodGet, "/api/hosts/"+h.ID+"/nodes/node-dyn-01/client-config"))
	if err != nil || res1.StatusCode != http.StatusOK {
		t.Fatalf("failed initial request: status %d", res1.StatusCode)
	}
	var data1 proxy.AllClientConfigResponse
	_ = json.NewDecoder(res1.Body).Decode(&data1)
	if !data1.Available || len(data1.Nodes) == 0 || len(data1.Nodes[0].Profiles) == 0 {
		t.Fatalf("expected available config with profiles on step 1: %+v", data1)
	}

	// 2. Stop Xray runtime on agent!
	atomic.StoreInt32(&xrayOnline, 0)

	// 3. Subsequent request -> Manager MUST immediately detect runtime unavailable and purge stale cache!
	res2, err := client.Do(authReq(http.MethodGet, "/api/hosts/"+h.ID+"/nodes/node-dyn-01/client-config"))
	if err != nil || res2.StatusCode != http.StatusOK {
		t.Fatalf("failed request after stop: status %d", res2.StatusCode)
	}
	var data2 proxy.AllClientConfigResponse
	_ = json.NewDecoder(res2.Body).Decode(&data2)

	if data2.Available {
		t.Fatalf("STALE CACHE VIOLATION: Manager returned available=true even though Xray stopped!")
	}
	if !strings.Contains(data2.Error, "unavailable") {
		t.Errorf("expected error message to specify unavailable: %s", data2.Error)
	}
	if len(data2.Profiles) > 0 || (len(data2.Nodes) > 0 && len(data2.Nodes[0].Profiles) > 0) {
		t.Fatalf("STALE CACHE VIOLATION: Stale VLESS profiles returned when runtime is unavailable!")
	}
}

