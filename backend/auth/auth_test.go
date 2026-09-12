package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	tempDir, err := os.MkdirTemp("", "spm_auth_test_*")
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

func TestBootstrapAndForcedPasswordChange(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Initial bootstrap should generate password and set must_change_password=true
	bootstrapped, tempPass, err := CheckOrInitBootstrap(database, "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("CheckOrInitBootstrap failed: %v", err)
	}
	if !bootstrapped {
		t.Fatalf("Expected bootstrapped to be true on clean DB")
	}
	if len(tempPass) < 16 {
		t.Fatalf("Temporary password too short: %s", tempPass)
	}

	// 2. Second bootstrap attempt must NOT regenerate password
	bootstrapped2, tempPass2, err := CheckOrInitBootstrap(database, "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("Second CheckOrInitBootstrap failed: %v", err)
	}
	if bootstrapped2 {
		t.Fatalf("Expected second bootstrap to be false")
	}
	if tempPass2 != "" {
		t.Fatalf("Expected empty temp password on second run, got: %s", tempPass2)
	}

	// 3. Login with temporary password
	user, err := Authenticate(database, "admin", tempPass)
	if err != nil {
		t.Fatalf("Authenticate with temp password failed: %v", err)
	}
	if !user.MustChangePassword {
		t.Fatalf("Expected MustChangePassword to be true")
	}

	// 4. Create session with temporary password
	sess, err := CreateSession(database, user, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if !sess.MustChangePassword {
		t.Fatalf("Expected session MustChangePassword to be true")
	}

	// 5. Test middleware blocks access to normal API endpoints
	normalHandler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	// Request to normal endpoint with temp session should return 403 Forbidden
	req := httptest.NewRequest("GET", "/api/daemon/status", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sess.Token})
	rr := httptest.NewRecorder()

	SessionMiddleware(database)(normalHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden on /api/daemon/status with temp session, got: %d", rr.Code)
	}

	// Allowed endpoints during password change required: /api/auth/me, /api/auth/change-password
	reqMe := httptest.NewRequest("GET", "/api/auth/me", nil)
	reqMe.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sess.Token})
	rrMe := httptest.NewRecorder()

	SessionMiddleware(database)(normalHandler).ServeHTTP(rrMe, reqMe)
	if rrMe.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK on /api/auth/me with temp session, got: %d", rrMe.Code)
	}

	// 6. Test Password Change Policies
	// Policy 1: Too short (<12 chars)
	err = ChangePassword(database, "admin", tempPass, "Short1!", "Short1!")
	if err == nil {
		t.Fatalf("Expected error for short password (<12 chars)")
	}

	// Policy 2: Mismatch
	err = ChangePassword(database, "admin", tempPass, "ValidNewPass2026!", "DifferentPass2026!")
	if err == nil {
		t.Fatalf("Expected error for mismatched confirmation password")
	}

	// Policy 3: Same as temporary password
	err = ChangePassword(database, "admin", tempPass, tempPass, tempPass)
	if err == nil {
		t.Fatalf("Expected error when new password equals temporary password")
	}

	// Policy 4: Wrong current password
	err = ChangePassword(database, "admin", "WrongOldPassword!", "ValidNewPass2026!", "ValidNewPass2026!")
	if err == nil {
		t.Fatalf("Expected error for incorrect old password")
	}

	// 7. Successful Password Change
	newPassword := "SecureProductionPass2026#!"
	err = ChangePassword(database, "admin", tempPass, newPassword, newPassword)
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// 8. Old temporary password must now be rejected
	_, err = Authenticate(database, "admin", tempPass)
	if err == nil {
		t.Fatalf("Expected old temp password to be rejected after change")
	}

	// 9. New password works and has must_change_password=false
	userUpdated, err := Authenticate(database, "admin", newPassword)
	if err != nil {
		t.Fatalf("Authenticate with new password failed: %v", err)
	}
	if userUpdated.MustChangePassword {
		t.Fatalf("Expected MustChangePassword to be false after change")
	}

	// 10. Previous session should be revoked
	_, valid := GetSession(database, sess.Token)
	if valid {
		t.Fatalf("Expected old session to be invalidated after password change")
	}

	// 11. Create new session with updated user and verify full access
	sessNew, err := CreateSession(database, userUpdated, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession for new user failed: %v", err)
	}

	reqAllowed := httptest.NewRequest("GET", "/api/daemon/status", nil)
	reqAllowed.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessNew.Token})
	rrAllowed := httptest.NewRecorder()

	SessionMiddleware(database)(normalHandler).ServeHTTP(rrAllowed, reqAllowed)
	if rrAllowed.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK after password change, got: %d", rrAllowed.Code)
	}
}

func TestResetAdminPasswordPreservesDatabase(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	_, initialPassword, err := CheckOrInitBootstrap(database, "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	updatedPassword := "ExistingPassword2026!"
	if err := ChangePassword(database, "admin", initialPassword, updatedPassword, updatedPassword); err != nil {
		t.Fatalf("initial password change failed: %v", err)
	}
	updatedUser, err := Authenticate(database, "admin", updatedPassword)
	if err != nil {
		t.Fatalf("authenticate before reset failed: %v", err)
	}
	oldSession, err := CreateSession(database, updatedUser, time.Hour)
	if err != nil {
		t.Fatalf("create session before reset failed: %v", err)
	}
	if _, err := database.Conn().Exec(`
		INSERT INTO audit_logs (timestamp, username, role, action, target, result, source_ip, details)
		VALUES (?, 'admin', 'admin', 'test', 'preserved-row', 'success', '127.0.0.1', 'before reset')
	`, time.Now().UTC()); err != nil {
		t.Fatalf("insert preserved row failed: %v", err)
	}

	resetPassword, err := ResetAdminPassword(database)
	if err != nil {
		t.Fatalf("ResetAdminPassword failed: %v", err)
	}
	if resetPassword == "" || resetPassword == updatedPassword {
		t.Fatal("reset must generate a new temporary password")
	}
	if _, err := Authenticate(database, "admin", updatedPassword); err == nil {
		t.Fatal("previous password must be rejected after reset")
	}
	user, err := Authenticate(database, "admin", resetPassword)
	if err != nil {
		t.Fatalf("temporary reset password must authenticate: %v", err)
	}
	if !user.MustChangePassword {
		t.Fatal("reset password must require a password change")
	}
	if _, valid := GetSession(database, oldSession.Token); valid {
		t.Fatal("reset must revoke existing admin sessions")
	}
	var preservedRows int
	if err := database.Conn().QueryRow("SELECT COUNT(*) FROM audit_logs WHERE target = 'preserved-row'").Scan(&preservedRows); err != nil {
		t.Fatalf("query preserved row failed: %v", err)
	}
	if preservedRows != 1 {
		t.Fatalf("reset changed unrelated database records: got %d preserved rows", preservedRows)
	}
}

func TestCSRFValidation(t *testing.T) {
	sess := &Session{CSRFToken: "valid-csrf-token-12345"}
	if !ValidateCSRF(sess, "valid-csrf-token-12345") {
		t.Errorf("Expected CSRF validation to succeed")
	}
	if ValidateCSRF(sess, "invalid-token") {
		t.Errorf("Expected CSRF validation to fail")
	}
	if ValidateCSRF(nil, "token") {
		t.Errorf("Expected CSRF validation on nil session to fail")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(10, 2)
	ip := "192.168.1.100"

	l := limiter.GetLimiter(ip)
	if !l.Allow() {
		t.Errorf("First request should be allowed")
	}
	if !l.Allow() {
		t.Errorf("Second burst request should be allowed")
	}
}
