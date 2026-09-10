package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDB_InitAndEncryption(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "spm_db_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := InitDB(tempDir)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify key file was created
	keyFile := filepath.Join(tempDir, ".master.key")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Errorf("Expected .master.key to exist")
	}

	// Test encryption & decryption
	plain := "secret-super-proxy-agent-token-12345"
	enc, err := db.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if enc == plain {
		t.Errorf("Ciphertext equals plaintext")
	}

	dec, err := db.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if dec != plain {
		t.Errorf("Expected %q, got %q", plain, dec)
	}

	// Test empty string
	encEmpty, err := db.Encrypt("")
	if err != nil || encEmpty != "" {
		t.Errorf("Expected empty encrypted string, got %q, err: %v", encEmpty, err)
	}
	decEmpty, err := db.Decrypt("")
	if err != nil || decEmpty != "" {
		t.Errorf("Expected empty decrypted string, got %q, err: %v", decEmpty, err)
	}
}
