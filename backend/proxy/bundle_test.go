package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSchemaV1GatewayBundle(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/client-config/all" || r.Header.Get("Authorization") != "Bearer integration-token" {
			t.Error("incorrect daemon request")
		}
		fmt.Fprint(w, `{"schema_version":1,"generated_at":"2026-09-11T00:00:00Z","node":{"id":"gateway","country":"JP"},"endpoint":{"address":"proxy.example.com","port":443,"network":"tcp","protocol":"vless","tls":true},"reality":{"flow":"xtls-rprx-vision"},"profiles":[{"id":"vless","format":"uri","filename":"vless.txt","content":"vless://exact-content"}]}`)
	}))
	defer srv.Close()
	c := NewClientWithFingerprint(srv.URL, "integration-token", FormatFingerprint(srv.Certificate().Raw))
	for _, id := range []string{"", "selected-exit"} {
		result, err := c.GetAllClientConfig(id)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Available || len(result.Nodes) != 1 {
			t.Fatalf("bundle marked unavailable: %+v", result)
		}
		n := result.Nodes[0]
		if n.EndpointAddress != "proxy.example.com" || n.EndpointPort != 443 || n.Country != "JP" || !n.Profiles[0].CanQR || n.Profiles[0].Content != "vless://exact-content" {
			t.Fatalf("lost endpoint/profile data: %+v", n)
		}
		if id == "" && n.NodeID != "gateway" {
			t.Fatal("gateway identity missing")
		}
		if id != "" && n.NodeID != id {
			t.Fatal("selected node identity missing")
		}
	}
}
