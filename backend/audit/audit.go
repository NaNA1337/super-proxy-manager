package audit

import (
	"net/http"
	"sync"
	"time"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
)

type AuditEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	Role      string    `json:"role"`
	Action    string    `json:"action"` // "login", "switch_slot", "create_subscription", "revoke_subscription"
	Target    string    `json:"target"`
	Result    string    `json:"result"` // "SUCCESS", "FAILED", "FORBIDDEN"
	SourceIP  string    `json:"source_ip"`
	Details   string    `json:"details,omitempty"`
}

type Logger struct {
	mu      sync.RWMutex
	nextID  int64
	entries []*AuditEntry
	maxSize int
}

var GlobalLogger = NewLogger(1000)

func NewLogger(maxSize int) *Logger {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &Logger{
		entries: make([]*AuditEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (l *Logger) Log(user, role, action, target, result, ip, details string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextID++
	entry := &AuditEntry{
		ID:        l.nextID,
		Timestamp: time.Now(),
		User:      user,
		Role:      role,
		Action:    action,
		Target:    target,
		Result:    result,
		SourceIP:  ip,
		Details:   details,
	}

	if len(l.entries) >= l.maxSize {
		// Evict oldest
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
}

func (l *Logger) GetEntries() []*AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	res := make([]*AuditEntry, len(l.entries))
	// Return in reverse chronological order (newest first)
	for i, j := 0, len(l.entries)-1; j >= 0; i, j = i+1, j-1 {
		res[i] = l.entries[j]
	}
	return res
}

func HandleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := GlobalLogger.GetEntries()
	auth.SendJSON(w, http.StatusOK, entries)
}
