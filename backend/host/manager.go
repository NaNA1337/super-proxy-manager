package host

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/proxy"
)

type HostManager struct {
	db      *db.DB
	mu      sync.RWMutex
	clients map[string]*proxy.DaemonClient
}

func NewHostManager(database *db.DB) *HostManager {
	return &HostManager{
		db:      database,
		clients: make(map[string]*proxy.DaemonClient),
	}
}

// ListHosts returns all configured hosts with sanitized attributes (no token)
func (m *HostManager) ListHosts() ([]*Host, error) {
	rows, err := m.db.Conn().Query(`
		SELECT id, name, address, agent_url, enabled, status, last_seen, version, region, tls_fingerprint, is_default, created_at, updated_at
		FROM manager_hosts
		ORDER BY is_default DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query hosts: %w", err)
	}
	defer rows.Close()

	var hosts []*Host
	for rows.Next() {
		var h Host
		var lastSeen sql.NullTime
		var isDefInt, enInt int

		err := rows.Scan(
			&h.ID, &h.Name, &h.Address, &h.AgentURL,
			&enInt, &h.Status, &lastSeen, &h.Version, &h.Region, &h.TLSFingerprint, &isDefInt,
			&h.CreatedAt, &h.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		h.Enabled = (enInt == 1)
		h.IsDefault = (isDefInt == 1)
		if lastSeen.Valid {
			h.LastSeen = lastSeen.Time
		}

		hosts = append(hosts, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hosts, nil
}

// GetHost retrieves a single host by ID
func (m *HostManager) GetHost(id string) (*Host, error) {
	var h Host
	var lastSeen sql.NullTime
	var isDefInt, enInt int

	err := m.db.Conn().QueryRow(`
		SELECT id, name, address, agent_url, enabled, status, last_seen, version, region, tls_fingerprint, is_default, created_at, updated_at
		FROM manager_hosts
		WHERE id = ?
	`, id).Scan(
		&h.ID, &h.Name, &h.Address, &h.AgentURL,
		&enInt, &h.Status, &lastSeen, &h.Version, &h.Region, &h.TLSFingerprint, &isDefInt,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("host not found")
		}
		return nil, err
	}

	h.Enabled = (enInt == 1)
	h.IsDefault = (isDefInt == 1)
	if lastSeen.Valid {
		h.LastSeen = lastSeen.Time
	}

	return &h, nil
}

// GetDefaultHost returns the currently active default host
func (m *HostManager) GetDefaultHost() (*Host, error) {
	var h Host
	var lastSeen sql.NullTime
	var isDefInt, enInt int

	err := m.db.Conn().QueryRow(`
		SELECT id, name, address, agent_url, enabled, status, last_seen, version, region, tls_fingerprint, is_default, created_at, updated_at
		FROM manager_hosts
		WHERE is_default = 1 AND enabled = 1
		LIMIT 1
	`).Scan(
		&h.ID, &h.Name, &h.Address, &h.AgentURL,
		&enInt, &h.Status, &lastSeen, &h.Version, &h.Region, &h.TLSFingerprint, &isDefInt,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Fallback to first enabled host
			hosts, lErr := m.ListHosts()
			if lErr == nil && len(hosts) > 0 {
				for _, candidate := range hosts {
					if candidate.Enabled {
						return candidate, nil
					}
				}
				return hosts[0], nil
			}
			return nil, errors.New("no hosts configured")
		}
		return nil, err
	}

	h.Enabled = (enInt == 1)
	h.IsDefault = (isDefInt == 1)
	if lastSeen.Valid {
		h.LastSeen = lastSeen.Time
	}

	return &h, nil
}

// SetDefaultHost sets specified host as default and unsets all others
func (m *HostManager) SetDefaultHost(id string) error {
	tx, err := m.db.Conn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE manager_hosts SET is_default = 0")
	if err != nil {
		return err
	}

	res, err := tx.Exec("UPDATE manager_hosts SET is_default = 1 WHERE id = ?", id)
	if err != nil {
		return err
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return errors.New("host not found")
	}

	return tx.Commit()
}

// CreateHost validates, encrypts credentials, pins TLS fingerprint, and adds a new host
func (m *HostManager) CreateHost(name, address, agentURL, token string, isDefault bool) (*Host, error) {
	return m.CreateHostWithFingerprint(name, address, agentURL, token, "", isDefault)
}

// CreateHostWithFingerprint creates host with specific TLS fingerprint
func (m *HostManager) CreateHostWithFingerprint(name, address, agentURL, token, tlsFingerprint string, isDefault bool) (*Host, error) {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	agentURL = strings.TrimSpace(agentURL)
	token = strings.TrimSpace(token)
	tlsFingerprint = strings.TrimSpace(tlsFingerprint)

	if name == "" {
		return nil, errors.New("host name is required")
	}
	if address == "" {
		return nil, errors.New("host address is required")
	}
	if err := ValidateAgentURL(agentURL); err != nil {
		return nil, fmt.Errorf("invalid agent URL: %w", err)
	}

	// Auto-probe TLS fingerprint if not provided and scheme is HTTPS
	if tlsFingerprint == "" && strings.HasPrefix(strings.ToLower(agentURL), "https://") {
		detectedFP, err := proxy.ProbeTLS(agentURL)
		if err == nil && detectedFP != "" {
			tlsFingerprint = detectedFP
		}
	}

	// Encrypt token
	encryptedToken, err := m.db.Encrypt(token)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Check if this is the first host
	var count int
	_ = m.db.Conn().QueryRow("SELECT COUNT(*) FROM manager_hosts").Scan(&count)
	if count == 0 {
		isDefault = true
	}

	// Generate ID
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	id := fmt.Sprintf("host-%s", hex.EncodeToString(randBytes))

	now := time.Now().UTC()

	tx, err := m.db.Conn().Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if isDefault {
		_, _ = tx.Exec("UPDATE manager_hosts SET is_default = 0")
	}

	isDefInt := 0
	if isDefault {
		isDefInt = 1
	}

	_, err = tx.Exec(`
		INSERT INTO manager_hosts (id, name, address, agent_url, encrypted_token, enabled, status, tls_fingerprint, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, 'pending', ?, ?, ?, ?)
	`, id, name, address, agentURL, encryptedToken, tlsFingerprint, isDefInt, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert host: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Cache client instance
	client := proxy.NewClientWithFingerprint(agentURL, token, tlsFingerprint)
	m.mu.Lock()
	m.clients[id] = client
	m.mu.Unlock()

	// Probe host in background to populate version and status
	go func() {
		_, _ = m.ProbeHost(id)
	}()

	return m.GetHost(id)
}

// UpdateHost updates host properties and optionally replaces the token / TLS fingerprint
func (m *HostManager) UpdateHost(id, name, address, agentURL, token string, enabled bool) (*Host, error) {
	return m.UpdateHostWithFingerprint(id, name, address, agentURL, token, "", enabled)
}

// UpdateHostWithFingerprint updates host properties with explicit TLS fingerprint
func (m *HostManager) UpdateHostWithFingerprint(id, name, address, agentURL, token, tlsFingerprint string, enabled bool) (*Host, error) {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	agentURL = strings.TrimSpace(agentURL)
	token = strings.TrimSpace(token)
	tlsFingerprint = strings.TrimSpace(tlsFingerprint)

	if name == "" {
		return nil, errors.New("host name cannot be empty")
	}
	if address == "" {
		return nil, errors.New("host address cannot be empty")
	}
	if err := ValidateAgentURL(agentURL); err != nil {
		return nil, fmt.Errorf("invalid agent URL: %w", err)
	}

	now := time.Now().UTC()
	enInt := 0
	if enabled {
		enInt = 1
	}

	// If fingerprint not provided, keep existing
	if tlsFingerprint == "" {
		_ = m.db.Conn().QueryRow("SELECT tls_fingerprint FROM manager_hosts WHERE id = ?", id).Scan(&tlsFingerprint)
	}

	if token != "" {
		encryptedToken, err := m.db.Encrypt(token)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt token: %w", err)
		}
		_, err = m.db.Conn().Exec(`
			UPDATE manager_hosts
			SET name = ?, address = ?, agent_url = ?, encrypted_token = ?, tls_fingerprint = ?, enabled = ?, updated_at = ?
			WHERE id = ?
		`, name, address, agentURL, encryptedToken, tlsFingerprint, enInt, now, id)
		if err != nil {
			return nil, err
		}

		// Update cached client
		m.mu.Lock()
		m.clients[id] = proxy.NewClientWithFingerprint(agentURL, token, tlsFingerprint)
		m.mu.Unlock()
	} else {
		_, err := m.db.Conn().Exec(`
			UPDATE manager_hosts
			SET name = ?, address = ?, agent_url = ?, tls_fingerprint = ?, enabled = ?, updated_at = ?
			WHERE id = ?
		`, name, address, agentURL, tlsFingerprint, enInt, now, id)
		if err != nil {
			return nil, err
		}

		// Invalidate cached client to force re-instantiation with updated URL/fingerprint
		m.mu.Lock()
		delete(m.clients, id)
		m.mu.Unlock()
	}

	go func() {
		_, _ = m.ProbeHost(id)
	}()

	return m.GetHost(id)
}

// DeleteHost removes a host from database and client pool
func (m *HostManager) DeleteHost(id string) error {
	host, err := m.GetHost(id)
	if err != nil {
		return err
	}

	var count int
	_ = m.db.Conn().QueryRow("SELECT COUNT(*) FROM manager_hosts").Scan(&count)

	if host.IsDefault && count > 1 {
		return errors.New("cannot delete default host; please select another host as default first")
	}

	_, err = m.db.Conn().Exec("DELETE FROM manager_hosts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}

	m.mu.Lock()
	delete(m.clients, id)
	m.mu.Unlock()

	return nil
}

// GetClient returns a thread-safe DaemonClient for the requested host
func (m *HostManager) GetClient(hostID string) (*proxy.DaemonClient, error) {
	if hostID == "" || hostID == "default" {
		def, err := m.GetDefaultHost()
		if err != nil {
			return nil, err
		}
		hostID = def.ID
	}

	m.mu.RLock()
	client, exists := m.clients[hostID]
	m.mu.RUnlock()
	if exists && client != nil {
		return client, nil
	}

	// Fetch from DB
	var agentURL, encToken, tlsFingerprint string
	var enabled int
	err := m.db.Conn().QueryRow(`
		SELECT agent_url, encrypted_token, enabled, tls_fingerprint
		FROM manager_hosts
		WHERE id = ?
	`, hostID).Scan(&agentURL, &encToken, &enabled, &tlsFingerprint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("host %q not found", hostID)
		}
		return nil, err
	}

	if enabled == 0 {
		return nil, fmt.Errorf("host %q is disabled", hostID)
	}

	token, err := m.db.Decrypt(encToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt host token: %w", err)
	}

	client = proxy.NewClientWithFingerprint(agentURL, token, tlsFingerprint)
	m.mu.Lock()
	m.clients[hostID] = client
	m.mu.Unlock()

	return client, nil
}

// TestConnection tests reachability, TLS fingerprint, and credentials for an agent URL and token without saving
func (m *HostManager) TestConnection(agentURL, token string) (*TestConnectionResult, error) {
	if err := ValidateAgentURL(agentURL); err != nil {
		return &TestConnectionResult{
			Success: false,
			Status:  "incompatible",
			Error:   err.Error(),
		}, nil
	}

	// Probe TLS certificate fingerprint if HTTPS
	detectedFP, probeErr := proxy.ProbeTLS(agentURL)
	if probeErr != nil && strings.HasPrefix(strings.ToLower(agentURL), "https://") {
		return &TestConnectionResult{
			Success: false,
			Status:  "offline",
			Error:   fmt.Sprintf("TLS probe failed: %v", probeErr),
		}, nil
	}

	client := proxy.NewClientWithFingerprint(agentURL, token, detectedFP)
	start := time.Now()

	status, err := client.GetStatus()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		safeErr := err.Error()
		if strings.Contains(safeErr, "x509") || strings.Contains(safeErr, "fingerprint mismatch") {
			safeErr = "TLS certificate validation failed: " + safeErr
		} else if strings.Contains(safeErr, "connection refused") {
			safeErr = "Connection refused: daemon is not reachable on target port"
		} else if strings.Contains(safeErr, "401") || strings.Contains(safeErr, "Unauthorized") {
			safeErr = "Authentication failed: invalid Agent API token"
		}
		return &TestConnectionResult{
			Success:             false,
			Status:              "offline",
			LatencyMs:           latency,
			DetectedFingerprint: detectedFP,
			Error:               safeErr,
		}, nil
	}

	version := "1.0.0"
	if v, ok := status["version"].(string); ok && v != "" {
		version = v
	}
	region := ""
	if r, ok := status["region"].(string); ok {
		region = r
	} else if r, ok := status["server_id"].(string); ok {
		region = r
	}

	return &TestConnectionResult{
		Success:             true,
		Status:              "online",
		Version:             version,
		Region:              region,
		LatencyMs:           latency,
		DetectedFingerprint: detectedFP,
		TLSFingerprint:      detectedFP,
	}, nil
}

// ProbeHost checks an existing host and records status and telemetry in database
func (m *HostManager) ProbeHost(hostID string) (*TestConnectionResult, error) {
	var agentURL, encToken, tlsFingerprint string
	err := m.db.Conn().QueryRow(`
		SELECT agent_url, encrypted_token, tls_fingerprint
		FROM manager_hosts
		WHERE id = ?
	`, hostID).Scan(&agentURL, &encToken, &tlsFingerprint)
	if err != nil {
		return nil, err
	}

	token, err := m.db.Decrypt(encToken)
	if err != nil {
		return nil, err
	}

	res, err := m.TestConnection(agentURL, token)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if res.Success {
		// Update status to online, record version, region, and update TLS fingerprint if previously empty
		if tlsFingerprint == "" && res.DetectedFingerprint != "" {
			tlsFingerprint = res.DetectedFingerprint
		}
		_, _ = m.db.Conn().Exec(`
			UPDATE manager_hosts
			SET status = ?, version = ?, region = ?, tls_fingerprint = ?, last_seen = ?, updated_at = ?
			WHERE id = ?
		`, "online", res.Version, res.Region, tlsFingerprint, now, now, hostID)
	} else {
		// Determine status: offline or unknown
		status := "offline"
		if strings.Contains(res.Error, "incompatible") {
			status = "incompatible"
		} else if strings.Contains(res.Error, "degraded") {
			status = "degraded"
		} else if strings.Contains(res.Error, "timeout") {
			status = "unknown"
		}
		_, _ = m.db.Conn().Exec(`
			UPDATE manager_hosts
			SET status = ?, updated_at = ?
			WHERE id = ?
		`, status, now, hostID)
	}

	return res, nil
}
