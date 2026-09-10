package proxy

import (
	"sync"
	"time"
)

type cachedItem struct {
	data      *AllClientConfigResponse
	expiresAt time.Time
}

// ClientConfigCache provides a thread-safe, TTL-bounded cache for canonical client configs
// with immediate invalidation when runtime endpoints are unavailable.
type ClientConfigCache struct {
	mu    sync.RWMutex
	items map[string]cachedItem
	ttl   time.Duration
}

// NewClientConfigCache creates a cache with the specified TTL (default 30 seconds)
func NewClientConfigCache(ttl time.Duration) *ClientConfigCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &ClientConfigCache{
		items: make(map[string]cachedItem),
		ttl:   ttl,
	}
}

// Get retrieves cached config for hostID and nodeID if not expired
func (c *ClientConfigCache) Get(hostID, nodeID string) (*AllClientConfigResponse, bool) {
	key := hostID + ":" + nodeID
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.data, true
}

// Set stores data in the cache. If data indicates unavailable or error,
// any existing cache is immediately purged instead of stored.
func (c *ClientConfigCache) Set(hostID, nodeID string, data *AllClientConfigResponse) {
	if data == nil {
		return
	}

	key := hostID + ":" + nodeID

	// If runtime is unavailable, immediately invalidate and do NOT cache stale config
	if !data.Available || data.Error != "" {
		c.Invalidate(hostID, nodeID)
		return
	}

	c.mu.Lock()
	c.items[key] = cachedItem{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Invalidate removes cached entries for a node or all nodes of a host
func (c *ClientConfigCache) Invalidate(hostID, nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if nodeID != "" {
		delete(c.items, hostID+":"+nodeID)
		return
	}

	// Invalidate all entries for this host
	prefix := hostID + ":"
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}
