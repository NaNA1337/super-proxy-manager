package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleReadOnly Role = "readonly"
)

type User struct {
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"`
	Role               Role      `json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Session struct {
	Token              string    `json:"token"`
	Username           string    `json:"username"`
	Role               Role      `json:"role"`
	CSRFToken          string    `json:"csrf_token"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

var (
	sessMu   sync.RWMutex
	sessions = make(map[string]*Session)
)

func generateTemporaryPassword() (string, error) {
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(randBytes), nil
}

// CheckOrInitBootstrap checks if any user exists. If not, it generates a high-entropy
// temporary bootstrap password for the 'admin' user, saves it to the database with
// must_change_password = true, and displays the setup banner ONCE in stdout.
func CheckOrInitBootstrap(database *db.DB, listenAddr string) (bool, string, error) {
	conn := database.Conn()

	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, "", fmt.Errorf("failed to query users count: %w", err)
	}

	if count > 0 {
		return false, "", nil
	}

	// Generate 32-character high-entropy temporary password
	tempPassword, err := generateTemporaryPassword()
	if err != nil {
		return false, "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, "", fmt.Errorf("failed to hash bootstrap password: %w", err)
	}

	now := time.Now().UTC()

	// Insert admin user
	_, err = conn.Exec(`
		INSERT INTO users (username, password_hash, role, must_change_password, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?)
	`, "admin", string(hash), string(RoleAdmin), now, now)
	if err != nil {
		return false, "", fmt.Errorf("failed to insert bootstrap user: %w", err)
	}

	// Record bootstrap state
	_, _ = conn.Exec(`
		INSERT OR REPLACE INTO bootstrap_state (id, is_bootstrapped, created_at)
		VALUES (1, 1, ?)
	`, now)

	// Display banner once in stdout
	displayAddr := listenAddr
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "localhost" + displayAddr
	}
	fmt.Printf("\n========================================================\n")
	fmt.Printf("SUPER-PROXY MANAGER FIRST-TIME SETUP\n")
	fmt.Printf("========================================================\n")
	fmt.Printf("Admin username: admin\n")
	fmt.Printf("Temporary password: %s\n", tempPassword)
	fmt.Printf("IMPORTANT: This password will be invalidated after first password change.\n")
	fmt.Printf("Open: http://%s\n", displayAddr)
	fmt.Printf("========================================================\n\n")

	return true, tempPassword, nil
}

// ResetAdminPassword replaces the admin password with a new one-time password.
// It preserves the database, configured hosts, and encryption master key.
func ResetAdminPassword(database *db.DB) (string, error) {
	temporaryPassword, err := generateTemporaryPassword()
	if err != nil {
		return "", fmt.Errorf("failed to generate temporary password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(temporaryPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash temporary password: %w", err)
	}

	tx, err := database.Conn().Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin admin password reset: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE users
		SET password_hash = ?, must_change_password = 1, updated_at = ?
		WHERE username = 'admin'
	`, string(hash), time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("failed to update admin password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("failed to verify admin password update: %w", err)
	}
	if rows != 1 {
		return "", errors.New("admin user not found")
	}

	if _, err := tx.Exec("DELETE FROM sessions WHERE username = 'admin'"); err != nil {
		return "", fmt.Errorf("failed to revoke admin sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit admin password reset: %w", err)
	}

	sessMu.Lock()
	for token, session := range sessions {
		if session.Username == "admin" {
			delete(sessions, token)
		}
	}
	sessMu.Unlock()

	return temporaryPassword, nil
}

// Authenticate verifies username and password against SQLite database
func Authenticate(database *db.DB, username, password string) (*User, error) {
	conn := database.Conn()

	var user User
	var roleStr string
	var mustChange int

	err := conn.QueryRow(`
		SELECT username, password_hash, role, must_change_password, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username).Scan(&user.Username, &user.PasswordHash, &roleStr, &mustChange, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Constant time dummy compare to prevent username enumeration timing attacks
			_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO1234567890"), []byte(password))
			return nil, errors.New("invalid username or password")
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	user.Role = Role(roleStr)
	user.MustChangePassword = (mustChange == 1)

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return &user, nil
}

// ChangePassword enforces security policy and updates user password in SQLite
func ChangePassword(database *db.DB, username, currentPass, newPass, confirmPass string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("username cannot be empty")
	}
	if newPass != confirmPass {
		return errors.New("new password and confirmation do not match")
	}
	if len(newPass) < 12 {
		return errors.New("new password must be at least 12 characters")
	}
	if newPass == currentPass {
		return errors.New("new password cannot be the same as the current temporary password")
	}

	conn := database.Conn()

	var oldHash string
	err := conn.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&oldHash)
	if err != nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(currentPass)); err != nil {
		return errors.New("current password is incorrect")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC()
	_, err = conn.Exec(`
		UPDATE users
		SET password_hash = ?, must_change_password = 0, updated_at = ?
		WHERE username = ?
	`, string(newHash), now, username)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	// Complete bootstrap state
	_, _ = conn.Exec("UPDATE bootstrap_state SET completed_at = ? WHERE id = 1", now)

	// Invalidate all existing sessions for this user
	RevokeAllUserSessions(database, username)

	return nil
}

// CreateSession generates a session and stores it both in memory and in SQLite
func CreateSession(database *db.DB, user *User, duration time.Duration) (*Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(duration)

	sess := &Session{
		Token:              hex.EncodeToString(tokenBytes),
		Username:           user.Username,
		Role:               user.Role,
		CSRFToken:          hex.EncodeToString(csrfBytes),
		MustChangePassword: user.MustChangePassword,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
	}

	sessMu.Lock()
	sessions[sess.Token] = sess
	sessMu.Unlock()

	// Persist to SQLite
	mustChangeInt := 0
	if sess.MustChangePassword {
		mustChangeInt = 1
	}
	_, err := database.Conn().Exec(`
		INSERT INTO sessions (token, username, role, csrf_token, must_change_password, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, sess.Token, sess.Username, string(sess.Role), sess.CSRFToken, mustChangeInt, sess.CreatedAt, sess.ExpiresAt)
	if err != nil {
		log.Printf("[auth] Warning: failed to persist session to db: %v", err)
	}

	return sess, nil
}

// GetSession retrieves and validates session
func GetSession(database *db.DB, token string) (*Session, bool) {
	if token == "" {
		return nil, false
	}

	sessMu.RLock()
	sess, exists := sessions[token]
	sessMu.RUnlock()

	if exists {
		if time.Now().UTC().After(sess.ExpiresAt) {
			RevokeSession(database, token)
			return nil, false
		}
		return sess, true
	}

	// Try DB lookup (e.g. after graceful restart)
	var s Session
	var roleStr string
	var mustChange int
	err := database.Conn().QueryRow(`
		SELECT token, username, role, csrf_token, must_change_password, created_at, expires_at
		FROM sessions
		WHERE token = ?
	`, token).Scan(&s.Token, &s.Username, &roleStr, &s.CSRFToken, &mustChange, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, false
	}

	if time.Now().UTC().After(s.ExpiresAt) {
		RevokeSession(database, token)
		return nil, false
	}

	s.Role = Role(roleStr)
	s.MustChangePassword = (mustChange == 1)

	sessMu.Lock()
	sessions[s.Token] = &s
	sessMu.Unlock()

	return &s, true
}

// RevokeSession removes a session
func RevokeSession(database *db.DB, token string) {
	sessMu.Lock()
	delete(sessions, token)
	sessMu.Unlock()

	if database != nil {
		_, _ = database.Conn().Exec("DELETE FROM sessions WHERE token = ?", token)
	}
}

// RevokeAllUserSessions invalidates all sessions for a specific username
func RevokeAllUserSessions(database *db.DB, username string) {
	sessMu.Lock()
	for token, sess := range sessions {
		if sess.Username == username {
			delete(sessions, token)
		}
	}
	sessMu.Unlock()

	if database != nil {
		_, _ = database.Conn().Exec("DELETE FROM sessions WHERE username = ?", username)
	}
}

// ValidateCSRF validates CSRF token using constant-time comparison
func ValidateCSRF(sess *Session, token string) bool {
	if sess == nil || token == "" || sess.CSRFToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sess.CSRFToken), []byte(token)) == 1
}
