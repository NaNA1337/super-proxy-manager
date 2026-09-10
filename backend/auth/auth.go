package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleReadOnly Role = "readonly"
)

type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	CSRFToken string    `json:"csrf_token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

var (
	usersMu  sync.RWMutex
	users    = make(map[string]*User)
	sessMu   sync.RWMutex
	sessions = make(map[string]*Session)
)

func init() {
	// Initialize default admin and readonly accounts
	adminPass := os.Getenv("WEB_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "Admin@SuperProxy2026!"
	}
	readonlyPass := os.Getenv("WEB_READONLY_PASSWORD")
	if readonlyPass == "" {
		readonlyPass = "Viewer@SuperProxy2026!"
	}

	_ = CreateUser("admin", adminPass, RoleAdmin)
	_ = CreateUser("readonly", readonlyPass, RoleReadOnly)
}

// CreateUser registers or updates a user with bcrypt hashed password
func CreateUser(username, password string, role Role) error {
	if username == "" || password == "" {
		return errors.New("username and password cannot be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	usersMu.Lock()
	defer usersMu.Unlock()
	users[username] = &User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now(),
	}
	return nil
}

// Authenticate verifies username and password, returning user info if valid
func Authenticate(username, password string) (*User, error) {
	usersMu.RLock()
	user, exists := users[username]
	usersMu.RUnlock()

	if !exists {
		// Constant time comparison dummy to prevent username enumeration timing
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO1234567890"), []byte(password))
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return user, nil
}

// CreateSession generates a new cryptographically secure session
func CreateSession(user *User, duration time.Duration) (*Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		return nil, err
	}

	sess := &Session{
		Token:     hex.EncodeToString(tokenBytes),
		Username:  user.Username,
		Role:      user.Role,
		CSRFToken: hex.EncodeToString(csrfBytes),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
	}

	sessMu.Lock()
	sessions[sess.Token] = sess
	sessMu.Unlock()

	return sess, nil
}

// GetSession retrieves and validates an unexpired session
func GetSession(token string) (*Session, bool) {
	if token == "" {
		return nil, false
	}
	sessMu.RLock()
	sess, exists := sessions[token]
	sessMu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(sess.ExpiresAt) {
		RevokeSession(token)
		return nil, false
	}

	return sess, true
}

// RevokeSession deletes a session
func RevokeSession(token string) {
	sessMu.Lock()
	delete(sessions, token)
	sessMu.Unlock()
}

// ValidateCSRF validates CSRF token using constant-time comparison
func ValidateCSRF(sess *Session, token string) bool {
	if sess == nil || token == "" || sess.CSRFToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sess.CSRFToken), []byte(token)) == 1
}
