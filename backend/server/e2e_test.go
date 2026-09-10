package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
)

func TestE2E_FullFlow(t *testing.T) {
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
