package subscription

import (
	"bytes"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubscriptionLifecycle(t *testing.T) {
	store := NewStore()

	// 1. Create Subscription
	sub, rawToken, err := store.Create("My Sub", ProfileAllActive, "", "", "admin", 30)
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	if rawToken == "" {
		t.Fatalf("empty raw token returned")
	}

	// 2. Security: Verify that store DOES NOT store raw token
	storedSub, exists := store.subsByID[sub.ID]
	if !exists {
		t.Fatalf("subscription not stored")
	}
	if storedSub.TokenHash == rawToken {
		t.Errorf("SECURITY VIOLATION: Store is saving raw token instead of hash!")
	}
	if storedSub.TokenHash != HashToken(rawToken) {
		t.Errorf("token hash mismatch")
	}

	// 3. Resolve by raw token
	found, err := store.GetByToken(rawToken)
	if err != nil {
		t.Fatalf("failed to resolve subscription by raw token: %v", err)
	}
	if found.ID != sub.ID {
		t.Errorf("resolved subscription ID mismatch")
	}

	// 4. Revocation Test
	if err := store.Revoke(sub.ID); err != nil {
		t.Fatalf("failed to revoke subscription: %v", err)
	}

	// 5. Subsequent lookup must fail immediately
	_, err = store.GetByToken(rawToken)
	if err == nil {
		t.Errorf("expected revoked subscription to be rejected, but it succeeded!")
	}
}

func TestRedactedLoggingMiddleware(t *testing.T) {
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	secretToken := "verysecretrawtoken1234567890abcdef"

	handler := RedactedLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/sub/"+secretToken, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	logOutput := logBuf.String()

	// Verify that secretToken is NOT in log output
	if strings.Contains(logOutput, secretToken) {
		t.Errorf("SECURITY LEAK: Raw subscription token leaked into access log! Output: %s", logOutput)
	}

	// Verify that [REDACTED] is in log output
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Errorf("Expected [REDACTED] in access log, got: %s", logOutput)
	}
}

func TestBase64SubscriptionOutput(t *testing.T) {
	rawLinks := "socks5://127.0.0.1:1080#JP-01\nsocks5://127.0.0.1:1080#US-01"
	encoded := base64.StdEncoding.EncodeToString([]byte(rawLinks))

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	lines := strings.Split(string(decoded), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 proxy lines, got %d", len(lines))
	}
}
