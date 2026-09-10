package sharelink

import (
	"net/url"
	"strings"
	"testing"
)

func TestSOCKS5Generator(t *testing.T) {
	gen := &SOCKS5Generator{}
	node := NodeInfo{
		ID:      "1.2.3.4",
		IP:      "1.2.3.4",
		Country: "JP",
	}

	// 1. Unauthenticated SOCKS5
	cfg := RuntimeConfig{
		SocksAddress: "127.0.0.1",
		SocksPort:    1080,
	}
	uri, err := gen.Generate(node, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(uri, "socks5://127.0.0.1:1080#") {
		t.Errorf("unexpected URI: %s", uri)
	}

	// 2. Authenticated SOCKS5
	cfgAuth := RuntimeConfig{
		SocksAddress: "proxy.example.com",
		SocksPort:    1080,
		SocksUser:    "user_test",
		SocksPass:    "pass@123#special!",
	}
	uriAuth, err := gen.Generate(node, cfgAuth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(uriAuth, "user_test:") {
		t.Errorf("expected user in URI: %s", uriAuth)
	}
}

func TestVLESSRealityGeneratorAndPropertyTests(t *testing.T) {
	gen := &VLESSGenerator{}
	node := NodeInfo{
		ID:      "203.0.113.10",
		IP:      "203.0.113.10",
		Country: "JP",
	}

	cfg := RuntimeConfig{
		VlessEnabled:     true,
		VlessAddress:     "node.example.org",
		VlessPort:        443,
		VlessUUID:        "e96f131a-5b12-4d56-b072-4b7b2ce89012",
		VlessSecurity:    "reality",
		VlessSNI:         "gateway.icloud.com",
		VlessFingerprint: "chrome",
		VlessPublicKey:   "testPublicKeyBase64StringForRealityVerification123",
		VlessShortID:     "abcd1234ef",
		VlessFlow:        "xtls-rprx-vision",
		VlessType:        "tcp",
	}

	uri, err := gen.Generate(node, cfg)
	if err != nil {
		t.Fatalf("failed to generate VLESS reality link: %v", err)
	}

	// 1. Parse generated URI
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("generated URI is invalid url: %v", err)
	}

	// 2. Assert Scheme, User (UUID), Host, and Port
	if u.Scheme != "vless" {
		t.Errorf("expected scheme vless, got %s", u.Scheme)
	}
	if u.User.Username() != cfg.VlessUUID {
		t.Errorf("expected UUID %s, got %s", cfg.VlessUUID, u.User.Username())
	}
	if u.Hostname() != cfg.VlessAddress {
		t.Errorf("expected host %s, got %s", cfg.VlessAddress, u.Hostname())
	}
	if u.Port() != "443" {
		t.Errorf("expected port 443, got %s", u.Port())
	}

	// 3. Assert Query Parameters
	q := u.Query()
	if q.Get("security") != "reality" {
		t.Errorf("expected security=reality, got %s", q.Get("security"))
	}
	if q.Get("sni") != cfg.VlessSNI {
		t.Errorf("expected sni=%s, got %s", cfg.VlessSNI, q.Get("sni"))
	}
	if q.Get("pbk") != cfg.VlessPublicKey {
		t.Errorf("expected pbk=%s, got %s", cfg.VlessPublicKey, q.Get("pbk"))
	}
	if q.Get("sid") != cfg.VlessShortID {
		t.Errorf("expected sid=%s, got %s", cfg.VlessShortID, q.Get("sid"))
	}
	if q.Get("flow") != cfg.VlessFlow {
		t.Errorf("expected flow=%s, got %s", cfg.VlessFlow, q.Get("flow"))
	}
	if q.Get("type") != "tcp" {
		t.Errorf("expected type=tcp, got %s", q.Get("type"))
	}

	// 4. Security Check: Absolutely NO private key or secret leakage
	if strings.Contains(uri, "private") || strings.Contains(uri, "secret") {
		t.Errorf("SECURITY LEAK: private key or secret found in sharelink: %s", uri)
	}

	// 5. Test validator
	if err := ValidateGeneratedURI(uri); err != nil {
		t.Errorf("validator failed on valid URI: %v", err)
	}
}

func TestUnsupportedProtocols(t *testing.T) {
	svc := NewService()
	node := NodeInfo{ID: "1.1.1.1", IP: "1.1.1.1", Country: "US"}

	// When runtime has only SOCKS
	cfg := RuntimeConfig{
		SocksAddress: "127.0.0.1",
		SocksPort:    1080,
		VlessEnabled: false,
	}

	// VLESS should return Supported: false (not fabricating fake link)
	res, err := svc.Generate(node, "vless", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Supported {
		t.Errorf("expected VLESS to be NOT supported when disabled in runtime")
	}
	if res.URI != "" {
		t.Errorf("URI should be empty for unsupported protocol, got %s", res.URI)
	}

	// SOCKS5 should be supported
	resSocks, err := svc.Generate(node, "socks5", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resSocks.Supported || resSocks.URI == "" {
		t.Errorf("expected SOCKS5 to be supported and have URI")
	}
}

func TestSpecialCharactersAndUnicode(t *testing.T) {
	gen := &SOCKS5Generator{}
	node := NodeInfo{
		ID:      "192.0.2.1",
		IP:      "192.0.2.1",
		Country: "日本 🇯🇵 Tokyo",
	}
	cfg := RuntimeConfig{
		SocksAddress: "127.0.0.1",
		SocksPort:    1080,
	}

	uri, err := gen.Generate(node, cfg)
	if err != nil {
		t.Fatalf("failed to generate with unicode: %v", err)
	}

	if err := ValidateGeneratedURI(uri); err != nil {
		t.Errorf("failed validation on URI with unicode tag: %v", err)
	}
}
