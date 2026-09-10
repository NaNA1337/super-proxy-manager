package host

import (
	"time"
)

type Host struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	AgentURL    string    `json:"agent_url"`
	TokenRef    string    `json:"-"` // Never serialized or returned to frontend
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"` // "healthy", "degraded", "offline", "unauthorized", "incompatible"
	LastSeen    time.Time `json:"last_seen"`
	Version     string    `json:"version"`
	Region      string    `json:"region"`
	IsDefault   bool      `json:"is_default"`
	ActiveSlots int       `json:"active_slots"`
	LatencyMs   int64     `json:"latency_ms"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TestConnectionResult struct {
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	Region    string `json:"region,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}
