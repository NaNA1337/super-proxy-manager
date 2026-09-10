package host

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/NaNA1337/super-proxy-manager/backend/db"
)

func setupHostTestDB(t *testing.T) (*db.DB, func()) {
	tempDir, err := os.MkdirTemp("", "spm_host_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	database, err := db.InitDB(tempDir)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}
	return database, func() {
		database.Close()
		os.RemoveAll(tempDir)
	}
}

func TestSSRFValidation(t *testing.T) {
	// Ensure strict mode (no private hosts allowed)
	_ = os.Unsetenv("ALLOW_PRIVATE_HOSTS")
	_ = os.Unsetenv("SPM_ALLOW_PRIVATE_HOSTS")

	// Permitted public endpoints
	validURLs := []string{
		"https://node01.example.com:60000",
		"https://jp-proxy.superproxy.io:443",
		"http://93.184.216.34:8080",
	}
	for _, u := range validURLs {
		if err := ValidateAgentURL(u); err != nil {
			t.Errorf("Expected %q to be valid, got: %v", u, err)
		}
	}

	// Forbidden: Loopback & Localhost
	loopbackURLs := []string{
		"https://127.0.0.1:60000",
		"http://127.0.0.2:8080",
		"http://localhost:8080",
		"http://[::1]:8080",
	}
	for _, u := range loopbackURLs {
		if err := ValidateAgentURL(u); err == nil {
			t.Errorf("Expected loopback %q to be rejected by SSRF policy", u)
		}
	}

	// Forbidden: Private RFC1918 & Shared CIDRs
	privateURLs := []string{
		"http://10.0.0.1:8080",
		"http://172.16.0.1:8080",
		"http://192.168.1.1:8080",
		"http://169.254.1.1:8080",
		"http://100.64.0.1:8080",
		"http://[fc00::1]:8080",
		"http://[fe80::1]:8080",
	}
	for _, u := range privateURLs {
		if err := ValidateAgentURL(u); err == nil {
			t.Errorf("Expected private network %q to be rejected by SSRF policy", u)
		}
	}

	// Forbidden: Userinfo / embedded credentials
	if err := ValidateAgentURL("https://user:pass@example.com:60000"); err == nil {
		t.Errorf("Expected URL with userinfo to be rejected")
	}

	// Forbidden schemes
	forbiddenSchemes := []string{
		"file:///etc/passwd",
		"gopher://127.0.0.1:70",
		"ftp://ftp.example.com",
	}
	for _, u := range forbiddenSchemes {
		if err := ValidateAgentURL(u); err == nil {
			t.Errorf("Expected scheme %q to be rejected", u)
		}
	}

	// Forbidden cloud metadata
	metadataURLs := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://100.100.100.200/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://instance-data/latest/meta-data",
	}
	for _, u := range metadataURLs {
		if err := ValidateAgentURL(u); err == nil {
			t.Errorf("Expected metadata endpoint %q to be rejected", u)
		}
	}
}

func TestHostCRUDAndTokenEncryption(t *testing.T) {
	// Enable private mock server for unit testing
	_ = os.Setenv("ALLOW_PRIVATE_HOSTS", "true")
	defer os.Unsetenv("ALLOW_PRIVATE_HOSTS")

	database, cleanup := setupHostTestDB(t)
	defer cleanup()

	hm := NewHostManager(database)

	// Mock Daemon Server
	mockDaemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/status" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer secret-token-tokyo-01" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version":   "1.5.0",
				"server_id": "tokyo-datacenter-01",
				"uptime":    12345,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockDaemon.Close()

	// 1. Create Host
	secretToken := "secret-token-tokyo-01"
	h1, err := hm.CreateHost("Tokyo-01", "jp01.example.com", mockDaemon.URL, secretToken, false)
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// First host should automatically be marked default
	if !h1.IsDefault {
		t.Errorf("Expected first host to be default")
	}

	// Verify token is NOT exposed in Host struct
	if h1.TokenRef != "" {
		t.Errorf("TokenRef must not be exposed")
	}

	// Verify database stores encrypted token, NOT plaintext
	var encInDB string
	err = database.Conn().QueryRow("SELECT encrypted_token FROM manager_hosts WHERE id = ?", h1.ID).Scan(&encInDB)
	if err != nil {
		t.Fatalf("Failed to query encrypted_token from DB: %v", err)
	}
	if encInDB == secretToken {
		t.Fatalf("CRITICAL SECURITY FLAW: token stored in plaintext in DB!")
	}

	// 2. Test Connection
	testRes, err := hm.TestConnection(mockDaemon.URL, secretToken)
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if !testRes.Success {
		t.Fatalf("TestConnection should succeed: %v", testRes.Error)
	}
	if testRes.Version != "1.5.0" {
		t.Errorf("Expected version 1.5.0, got: %s", testRes.Version)
	}

	// Test Connection with invalid token
	failRes, _ := hm.TestConnection(mockDaemon.URL, "wrong-token")
	if failRes.Success {
		t.Errorf("Expected failure with wrong token")
	}

	// 3. Client Pool Retrieval
	client, err := hm.GetClient(h1.ID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus via retrieved client failed: %v", err)
	}
	if status["version"] != "1.5.0" {
		t.Errorf("Expected version 1.5.0, got: %v", status["version"])
	}

	// 4. Add second host
	h2, err := hm.CreateHost("Seoul-01", "kr01.example.com", "http://127.0.0.1:60001", "token-2", false)
	if err != nil {
		t.Fatalf("Create second host failed: %v", err)
	}
	if h2.IsDefault {
		t.Errorf("Second host should not be default unless specified")
	}

	// 5. Switch Default Host
	err = hm.SetDefaultHost(h2.ID)
	if err != nil {
		t.Fatalf("SetDefaultHost failed: %v", err)
	}
	defHost, err := hm.GetDefaultHost()
	if err != nil || defHost.ID != h2.ID {
		t.Fatalf("Expected default host to be %s, got: %v (err: %v)", h2.ID, defHost, err)
	}

	// 6. Delete Protection: Cannot delete default host if others exist
	err = hm.DeleteHost(h2.ID)
	if err == nil {
		t.Fatalf("Expected error when attempting to delete current default host")
	}

	// Switch default back to h1 and delete h2
	_ = hm.SetDefaultHost(h1.ID)
	err = hm.DeleteHost(h2.ID)
	if err != nil {
		t.Fatalf("Failed to delete non-default host: %v", err)
	}
}
