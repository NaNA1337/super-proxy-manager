package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAllClientConfigCanonical(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/client-config/all" {
			http.NotFound(w, r)
			return
		}

		nodeID := r.URL.Query().Get("node_id")
		if nodeID == "node-unavailable" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "runtime endpoint unavailable: Xray stopped",
			})
			return
		}

		resp := AllClientConfigResponse{
			Available: true,
			Nodes: []NodeClientConfig{
				{
					Available:       true,
					NodeID:          "node-tokyo-01",
					NodeIP:          "198.51.100.10",
					Country:         "JP",
					EndpointAddress: "jp01.example.com",
					EndpointPort:    443,
					Protocol:        "vless",
					Transport:       "tcp",
					Flow:            "xtls-rprx-vision",
					Profiles: []CanonicalProfile{
						{
							ID:          "vless",
							Name:        "VLESS URI",
							Format:      "uri",
							Filename:    "vless.txt",
							MimeType:    "text/plain",
							Content:     "vless://user@jp01.example.com:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=yahoo.co.jp#Tokyo-01",
							Description: "VLESS Reality / Vision",
							CanQR:       true,
						},
						{
							ID:       "clash",
							Name:     "Clash Meta",
							Format:   "yaml",
							Filename: "clash-meta.yaml",
							MimeType: "application/x-yaml",
							Content:  "proxies:\n  - name: Tokyo-01\n    type: vless\n    server: jp01.example.com",
							CanQR:    false,
						},
						{
							ID:       "sing-box",
							Name:     "sing-box",
							Format:   "json",
							Filename: "sing-box.json",
							MimeType: "application/json",
							Content:  `{"type":"vless","server":"jp01.example.com","server_port":443}`,
							CanQR:    false,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token")

	// 1. Successful query
	res, err := client.GetAllClientConfig("node-tokyo-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Available {
		t.Fatalf("expected available to be true")
	}
	if len(res.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(res.Nodes))
	}
	if len(res.Nodes[0].Profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(res.Nodes[0].Profiles))
	}

	// Verify exact profile formats
	vlessProfile := res.Nodes[0].Profiles[0]
	if vlessProfile.Format != "uri" || !vlessProfile.CanQR || !strings.HasPrefix(vlessProfile.Content, "vless://") {
		t.Errorf("vless profile mismatch: %+v", vlessProfile)
	}

	// 2. Unavailable node query
	resUnavail, err := client.GetAllClientConfig("node-unavailable")
	if err != nil {
		t.Fatalf("expected nil error on soft 503 response, got: %v", err)
	}
	if resUnavail.Available {
		t.Fatalf("expected available to be false on 503 unavailable")
	}
	if !strings.Contains(resUnavail.Error, "unavailable") {
		t.Errorf("expected error message to contain 'unavailable', got %q", resUnavail.Error)
	}
}

func TestNoFakeFallbackOnDaemonFailure(t *testing.T) {
	// A server that returns 500 internal server error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal daemon error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "token")

	// GetClientConfig must fail with error, NOT return 127.0.0.1:1080!
	_, err := client.GetClientConfig()
	if err == nil {
		t.Fatalf("GetClientConfig must fail when daemon returns 500; fake fallback was present!")
	}

	// GetRouting must fail with error, NOT synthesize fake state!
	_, err = client.GetRouting()
	if err == nil {
		t.Fatalf("GetRouting must fail when daemon returns 500; fake routing fallback was present!")
	}
}

func TestTLSFingerprintPinning(t *testing.T) {
	// Create HTTPS test server with self-signed certificate
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
	}))
	defer ts.Close()

	// Extract real certificate fingerprint
	realCert := ts.Certificate()
	realFingerprint := FormatFingerprint(realCert.Raw)

	// 1. Client configured with matching pinned fingerprint -> must SUCCEED
	validClient := NewClientWithFingerprint(ts.URL, "token", realFingerprint)
	// Point httpClient transport to use test server's listener
	status, err := validClient.GetStatus()
	if err != nil {
		t.Fatalf("expected successful connection with matching fingerprint: %v", err)
	}
	if status["status"] != "healthy" {
		t.Errorf("unexpected status: %+v", status)
	}

	// 2. Client configured with mismatched fingerprint -> must be BLOCKED!
	mismatchedFingerprint := "SHA256:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF"
	blockedClient := NewClientWithFingerprint(ts.URL, "token", mismatchedFingerprint)

	// Update transport to allow connecting to test server's self-signed cert, but our custom VerifyPeerCertificate will inspect fingerprint
	blockedClient.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
	_, err = blockedClient.GetStatus()
	if err == nil {
		t.Fatalf("connection with mismatched TLS certificate fingerprint was NOT blocked!")
	}
	if !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Errorf("expected error to mention 'fingerprint mismatch', got: %v", err)
	}
}
