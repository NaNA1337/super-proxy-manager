package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn      *sql.DB
	masterKey []byte
	mu        sync.RWMutex
}

var (
	globalDB *DB
	initOnce sync.Once
)

// InitDB initializes SQLite database and ensures schema is created
func InitDB(dataDir string) (*DB, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// Load or generate 32-byte master encryption key
	masterKey, err := loadOrGenerateMasterKey(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load master key: %w", err)
	}

	dbPath := filepath.Join(dataDir, "super-proxy-manager.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(time.Hour)

	db := &DB{
		conn:      conn,
		masterKey: masterKey,
	}

	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	globalDB = db
	return db, nil
}

// GetDB returns the global database instance
func GetDB() *DB {
	return globalDB
}

func (d *DB) Conn() *sql.DB {
	return d.conn
}

func (d *DB) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

func loadOrGenerateMasterKey(dataDir string) ([]byte, error) {
	// Check environment variable first
	if envKey := os.Getenv("MANAGER_MASTER_KEY"); envKey != "" {
		keyBytes, err := hex.DecodeString(envKey)
		if err == nil && len(keyBytes) == 32 {
			return keyBytes, nil
		}
		if len(envKey) >= 32 {
			return []byte(envKey[:32]), nil
		}
	}

	// Check persisted key file
	keyFile := filepath.Join(dataDir, ".master.key")
	if data, err := os.ReadFile(keyFile); err == nil {
		decoded, err := hex.DecodeString(string(data))
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}

	// Generate a new 32-byte key
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return nil, err
	}

	if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(newKey)), 0600); err != nil {
		return nil, fmt.Errorf("failed to persist master key file: %w", err)
	}

	return newKey, nil
}

// Encrypt encrypts plain text using AES-256-GCM
func (d *DB) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(d.masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (d *DB) Decrypt(encryptedHex string) (string, error) {
	if encryptedHex == "" {
		return "", nil
	}
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(d.masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS bootstrap_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		is_bootstrapped BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		completed_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		must_change_password BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		csrf_token TEXT NOT NULL,
		must_change_password BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS manager_hosts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		agent_url TEXT NOT NULL,
		encrypted_token TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'offline',
		last_seen TIMESTAMP,
		version TEXT NOT NULL DEFAULT '',
		region TEXT NOT NULL DEFAULT '',
		is_default BOOLEAN NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		action TEXT NOT NULL,
		target TEXT NOT NULL,
		result TEXT NOT NULL,
		source_ip TEXT NOT NULL,
		details TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS subscriptions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		profile TEXT NOT NULL,
		region TEXT NOT NULL DEFAULT '',
		protocol TEXT NOT NULL DEFAULT '',
		host_id TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		revoked BOOLEAN NOT NULL DEFAULT 0
	);
	`
	_, err := d.conn.Exec(schema)
	return err
}
