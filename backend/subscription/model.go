package subscription

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ProfileType string

const (
	ProfileAllActive ProfileType = "active_only"
	ProfileAllNodes  ProfileType = "all_nodes"
	ProfileRegion    ProfileType = "region"
	ProfileProtocol  ProfileType = "protocol"
)

type Subscription struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	TokenHash      string      `json:"-"`                         // DB only stores hash(token)
	HostID         string      `json:"host_id,omitempty"`         // Target Host ID or "all"
	Profile        ProfileType `json:"profile"`                   // active_only, all_nodes, region, protocol
	RegionFilter   string      `json:"region_filter,omitempty"`   // e.g. "JP", "US"
	ProtocolFilter string      `json:"protocol_filter,omitempty"` // e.g. "vless", "socks5"
	IsRevoked      bool        `json:"is_revoked"`
	CreatedAt      time.Time   `json:"created_at"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty"`
	CreatedBy      string      `json:"created_by"`
}

type CreateSubscriptionResponse struct {
	Subscription *Subscription `json:"subscription"`
	RawToken     string        `json:"raw_token"` // Displayed ONCE upon creation
	SubURL       string        `json:"sub_url"`
}

type Store struct {
	mu            sync.RWMutex
	subsByID      map[string]*Subscription
	subsByHash    map[string]*Subscription
}

func NewStore() *Store {
	return &Store{
		subsByID:   make(map[string]*Subscription),
		subsByHash: make(map[string]*Subscription),
	}
}

func HashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

func (s *Store) Create(name string, hostID string, profile ProfileType, region, protocol, username string, durationDays int) (*Subscription, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, "", err
	}
	rawToken := hex.EncodeToString(rawBytes)
	tokenHash := HashToken(rawToken)

	sub := &Subscription{
		ID:             uuid.New().String(),
		Name:           name,
		TokenHash:      tokenHash,
		HostID:         hostID,
		Profile:        profile,
		RegionFilter:   region,
		ProtocolFilter: protocol,
		IsRevoked:      false,
		CreatedAt:      time.Now(),
		CreatedBy:      username,
	}

	if durationDays > 0 {
		exp := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)
		sub.ExpiresAt = &exp
	}

	s.subsByID[sub.ID] = sub
	s.subsByHash[tokenHash] = sub

	return sub, rawToken, nil
}

func (s *Store) List() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Subscription, 0, len(s.subsByID))
	for _, sub := range s.subsByID {
		res = append(res, sub)
	}
	return res
}

func (s *Store) GetByToken(rawToken string) (*Subscription, error) {
	if rawToken == "" {
		return nil, errors.New("empty token")
	}
	tokenHash := HashToken(rawToken)

	s.mu.RLock()
	sub, exists := s.subsByHash[tokenHash]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("invalid token")
	}

	if sub.IsRevoked {
		return nil, errors.New("subscription has been revoked")
	}

	if sub.ExpiresAt != nil && time.Now().After(*sub.ExpiresAt) {
		return nil, errors.New("subscription has expired")
	}

	return sub, nil
}

func (s *Store) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, exists := s.subsByID[id]
	if !exists {
		return errors.New("subscription not found")
	}

	sub.IsRevoked = true
	return nil
}
