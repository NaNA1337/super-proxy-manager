package proxy

import (
	"testing"
	"time"
)

func TestClientConfigCacheHitAndExpiry(t *testing.T) {
	cache := NewClientConfigCache(50 * time.Millisecond)

	sampleConfig := &AllClientConfigResponse{
		Available: true,
		Nodes: []NodeClientConfig{
			{
				NodeID: "node-tokyo-01",
				Profiles: []CanonicalProfile{
					{ID: "vless", Name: "VLESS URI", Format: "uri", Content: "vless://test"},
				},
			},
		},
	}

	// 1. Initial get should miss
	if _, ok := cache.Get("host-1", "node-tokyo-01"); ok {
		t.Fatalf("expected cache miss for unpopulated key")
	}

	// 2. Set and verify hit
	cache.Set("host-1", "node-tokyo-01", sampleConfig)
	cached, ok := cache.Get("host-1", "node-tokyo-01")
	if !ok || cached == nil || len(cached.Nodes) == 0 {
		t.Fatalf("expected cache hit with populated data")
	}

	// 3. Verify isolation across hosts
	if _, ok := cache.Get("host-2", "node-tokyo-01"); ok {
		t.Fatalf("expected cache miss for different host ID (isolation violation)")
	}

	// 4. Wait for TTL expiry
	time.Sleep(60 * time.Millisecond)
	if _, ok := cache.Get("host-1", "node-tokyo-01"); ok {
		t.Fatalf("expected cache item to be expired after TTL")
	}
}

func TestClientConfigCacheImmediateInvalidationOnUnavailable(t *testing.T) {
	cache := NewClientConfigCache(10 * time.Second)

	positiveConfig := &AllClientConfigResponse{
		Available: true,
		Nodes: []NodeClientConfig{
			{
				NodeID: "node-1",
				Profiles: []CanonicalProfile{
					{ID: "vless", Content: "vless://active"},
				},
			},
		},
	}

	cache.Set("host-1", "node-1", positiveConfig)
	if _, ok := cache.Get("host-1", "node-1"); !ok {
		t.Fatalf("expected cache hit")
	}

	// Now runtime becomes unavailable: setting unavailable response must immediately invalidate
	unavailableConfig := &AllClientConfigResponse{
		Available: false,
		Error:     "runtime endpoint unavailable: Xray stopped",
	}

	cache.Set("host-1", "node-1", unavailableConfig)

	// Verify that the stale positive cache was immediately purged!
	if _, ok := cache.Get("host-1", "node-1"); ok {
		t.Fatalf("stale cache was NOT invalidated when runtime became unavailable!")
	}
}

func TestClientConfigCacheHostWideInvalidation(t *testing.T) {
	cache := NewClientConfigCache(10 * time.Second)

	cfg := &AllClientConfigResponse{Available: true}
	cache.Set("host-1", "node-a", cfg)
	cache.Set("host-1", "node-b", cfg)
	cache.Set("host-2", "node-a", cfg)

	// Invalidate all nodes of host-1
	cache.Invalidate("host-1", "")

	if _, ok := cache.Get("host-1", "node-a"); ok {
		t.Errorf("expected host-1:node-a to be invalidated")
	}
	if _, ok := cache.Get("host-1", "node-b"); ok {
		t.Errorf("expected host-1:node-b to be invalidated")
	}
	if _, ok := cache.Get("host-2", "node-a"); !ok {
		t.Errorf("host-2 should remain in cache")
	}
}
