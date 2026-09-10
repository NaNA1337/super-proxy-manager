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
	// Permitted
	if err := ValidateAgentURL("https://127.0.0.1:60000"); err != nil {
		t.Errorf("Expected 127.0.0.1:60000 to be valid, got: %v", err)
	}
	if err := ValidateAgentURL("http://localhost:8080"); err != nil {
		t.Errorf("Expected localhost:8080 to be valid, got: %v", err)
	}
	if err := ValidateAgentURL("https://node01.example.com:60000"); err != nil {
		t.Errorf("Expected domain:port to be valid, got: %v", err)
	}

	// Forbidden schemes
	if err := ValidateAgentURL("file:///etc/passwd"); err == nil {
		t.Errorf("Expected file:// to be rejected")
	}
	if err := ValidateAgentURL("gopher://127.0.0.1:70"); err == nil {
		t.Errorf("Expected gopher:// to be rejected")
	}
	if err := ValidateAgentURL("ftp://ftp.example.com"); err == nil {
		t.Errorf("Expected ftp:// to be rejected")
	}

	// Forbidden cloud metadata
	if err := ValidateAgentURL("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Errorf("Expected link-local metadata IP 169.254.169.254 to be rejected")
	}
	if err := ValidateAgentURL("http://metadata.google.internal/computeMetadata/v1/"); err == nil {
		t.Errorf("Expected metadata.google.internal to be rejected")
	}
	if err := ValidateAgentURL("http://instance-data/latest/meta-data"); err == nil {
		t.Errorf("Expected instance-data to be rejected")
	}
}

func TestHostCRUDAndTokenEncryption(t *testing.T) {
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
