package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticationAndSession(t *testing.T) {
	username := "testadmin"
	password := "SecretP@ssw0rd123!"

	if err := CreateUser(username, password, RoleAdmin); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Successful Auth
	user, err := Authenticate(username, password)
	if err != nil {
		t.Fatalf("expected successful authentication, got %v", err)
	}
	if user.Username != username || user.Role != RoleAdmin {
		t.Errorf("unexpected user details: %+v", user)
	}

	// Failed Auth (Wrong Password)
	_, err = Authenticate(username, "wrongpass")
	if err == nil {
		t.Errorf("expected authentication error with wrong password")
	}

	// Failed Auth (Nonexistent User)
	_, err = Authenticate("nonexistent", password)
	if err == nil {
		t.Errorf("expected authentication error with nonexistent user")
	}

	// Create Session
	sess, err := CreateSession(user, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if sess.Token == "" || sess.CSRFToken == "" {
		t.Errorf("empty session or csrf token")
	}

	// Retrieve Session
	retrieved, exists := GetSession(sess.Token)
	if !exists || retrieved.Username != username {
		t.Errorf("failed to retrieve session")
	}

	// CSRF Validation
	if !ValidateCSRF(sess, sess.CSRFToken) {
		t.Errorf("expected CSRF token to be valid")
	}
	if ValidateCSRF(sess, "invalid-csrf-token") {
		t.Errorf("expected invalid CSRF token to fail validation")
	}

	// Revoke Session
	RevokeSession(sess.Token)
	_, exists = GetSession(sess.Token)
	if exists {
		t.Errorf("session should no longer exist after revocation")
	}
}

func TestSessionExpiry(t *testing.T) {
	user := &User{Username: "expiring_user", Role: RoleReadOnly}
	sess, err := CreateSession(user, -1*time.Second) // expired
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	_, exists := GetSession(sess.Token)
	if exists {
		t.Errorf("expired session should not be returned")
	}
}

func TestRBACMiddleware(t *testing.T) {
	adminUser := &User{Username: "admin_test", Role: RoleAdmin}
	adminSess, _ := CreateSession(adminUser, 1*time.Hour)

	readonlyUser := &User{Username: "readonly_test", Role: RoleReadOnly}
	readonlySess, _ := CreateSession(readonlyUser, 1*time.Hour)

	adminHandler := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("admin_ok"))
	}))

	// 1. Unauthenticated -> 401
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	adminHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
	}

	// 2. Readonly user accessing admin -> 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: readonlySess.Token})
	rr = httptest.NewRecorder()
	SessionMiddleware(adminHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for readonly user on admin endpoint, got %d", rr.Code)
	}

	// 3. Admin user accessing admin -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSess.Token})
	rr = httptest.NewRecorder()
	SessionMiddleware(adminHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin user, got %d", rr.Code)
	}
}

func TestCSRFProtection(t *testing.T) {
	adminUser := &User{Username: "admin_csrf", Role: RoleAdmin}
	sess, _ := CreateSession(adminUser, 1*time.Hour)

	mutatingHandler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST without CSRF header -> 403
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sess.Token})
	rr := httptest.NewRecorder()
	SessionMiddleware(mutatingHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for mutating request without CSRF token, got %d", rr.Code)
	}

	// POST with valid CSRF header -> 200
	req = httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sess.Token})
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	rr = httptest.NewRecorder()
	SessionMiddleware(mutatingHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid CSRF token, got %d", rr.Code)
	}
}
