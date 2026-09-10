package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
)

func TestE2E_FullFlow(t *testing.T) {
	distFS, _ := embedded.GetDistFS()
	srv := NewServer("127.0.0.1:0", distFS)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. Test Static SPA Loading
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("failed to load root: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for root SPA, got %d", resp.StatusCode)
	}

	// 2. Failed Login Attempt
	badLoginBody := []byte(`{"username":"admin","password":"wrong-password"}`)
	resp, err = client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(badLoginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong password, got %d", resp.StatusCode)
	}

	// 3. Successful Login as Admin
	goodLoginBody := []byte(`{"username":"admin","password":"Admin@SuperProxy2026!"}`)
	resp, err = client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(goodLoginBody))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid admin login, got %d", resp.StatusCode)
	}

	var loginRes struct {
		Username  string `json:"username"`
		Role      string `json:"role"`
		Token     string `json:"token"`
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginRes); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginRes.Role != "admin" || loginRes.Token == "" || loginRes.CSRFToken == "" {
		t.Fatalf("invalid login response contents: %+v", loginRes)
	}

	// Get session cookie from response
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("missing session cookie in response")
	}

	// Helper for authenticated requests
	authReq := func(method, path string, body []byte) *http.Request {
		var r *http.Request
		if body != nil {
			r, _ = http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r, _ = http.NewRequest(method, ts.URL+path, nil)
		}
		r.AddCookie(sessionCookie)
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete {
			r.Header.Set("X-CSRF-Token", loginRes.CSRFToken)
		}
		return r
	}

	// 4. Test /api/auth/me
	meReq := authReq(http.MethodGet, "/api/auth/me", nil)
	meResp, err := client.Do(meReq)
	if err != nil || meResp.StatusCode != http.StatusOK {
		t.Fatalf("failed /api/auth/me: %v", err)
	}

	// 5. Test Supported ShareLink Protocols
	protoReq := authReq(http.MethodGet, "/api/sharelinks/protocols", nil)
	protoResp, err := client.Do(protoReq)
	if err != nil || protoResp.StatusCode != http.StatusOK {
		t.Fatalf("failed /api/sharelinks/protocols: %v", err)
	}
	var protoData struct {
		Supported []string `json:"supported"`
		All       []string `json:"all"`
	}
	_ = json.NewDecoder(protoResp.Body).Decode(&protoData)
	if len(protoData.All) < 5 {
		t.Errorf("expected all protocols list, got %+v", protoData)
	}

	// 6. Test Create Subscription
	subReqBody := []byte(`{"name":"E2E Test Sub","profile":"active_only","duration_days":30}`)
	createSubReq := authReq(http.MethodPost, "/api/subscriptions", subReqBody)
	createSubResp, err := client.Do(createSubReq)
	if err != nil || createSubResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create subscription: %v, status: %d", err, createSubResp.StatusCode)
	}

	var subRes struct {
		Subscription struct {
			ID string `json:"id"`
		} `json:"subscription"`
		RawToken string `json:"raw_token"`
		SubURL   string `json:"sub_url"`
	}
	_ = json.NewDecoder(createSubResp.Body).Decode(&subRes)
	if subRes.RawToken == "" || subRes.Subscription.ID == "" {
		t.Fatalf("invalid subscription response: %+v", subRes)
	}

	// 7. Access Public Subscription Endpoint
	subURL := ts.URL + "/sub/" + subRes.RawToken
	subResp, err := client.Get(subURL)
	if err != nil || subResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch subscription at %s: status %d", subURL, subResp.StatusCode)
	}
	subBytes := new(bytes.Buffer)
	_, _ = subBytes.ReadFrom(subResp.Body)
	_, err = base64.StdEncoding.DecodeString(subBytes.String())
	if err != nil {
		t.Fatalf("subscription payload is not valid base64: %v", err)
	}

	// 8. Revoke Subscription
	revokeReq := authReq(http.MethodPost, "/api/subscriptions/"+subRes.Subscription.ID+"/revoke", nil)
	revokeResp, err := client.Do(revokeReq)
	if err != nil || revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to revoke subscription: status %d", revokeResp.StatusCode)
	}

	// 9. Verify subsequent access is REJECTED (403)
	subRejectedResp, err := client.Get(subURL)
	if err != nil {
		t.Fatalf("failed to request revoked sub: %v", err)
	}
	if subRejectedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on revoked token, got %d", subRejectedResp.StatusCode)
	}

	// 10. Verify Mutation Audit Logs
	auditReq := authReq(http.MethodGet, "/api/audit", nil)
	auditResp, err := client.Do(auditReq)
	if err != nil || auditResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch audit logs: status %d", auditResp.StatusCode)
	}

	var auditEntries []map[string]interface{}
	_ = json.NewDecoder(auditResp.Body).Decode(&auditEntries)
	if len(auditEntries) < 2 {
		t.Fatalf("expected audit entries for login and subscription creation, got %d", len(auditEntries))
	}

	// Verify no passwords or secret tokens are leaked in audit logs
	for _, entry := range auditEntries {
		dataBytes, _ := json.Marshal(entry)
		entryStr := string(dataBytes)
		if strings.Contains(entryStr, "Admin@SuperProxy2026!") || strings.Contains(entryStr, subRes.RawToken) {
			t.Fatalf("SECURITY VIOLATION: Secret leaked in audit entry: %s", entryStr)
		}
	}
}
