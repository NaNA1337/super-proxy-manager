package sharelink

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SOCKS5Generator generates socks5:// links based on actual runtime SOCKS inbound
type SOCKS5Generator struct{}

func (g *SOCKS5Generator) Protocol() string { return "socks5" }

func (g *SOCKS5Generator) IsSupported(cfg RuntimeConfig) bool {
	return cfg.SocksPort > 0
}

func (g *SOCKS5Generator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("SOCKS5 is not configured in backend runtime")
	}

	addr := cfg.SocksAddress
	if addr == "" || addr == "0.0.0.0" || addr == "127.0.0.1" {
		addr = "127.0.0.1"
	}

	tag := fmt.Sprintf("super-proxy-%s-%s", node.Country, node.ID)
	if node.Country == "" {
		tag = fmt.Sprintf("super-proxy-%s", node.ID)
	}

	if cfg.SocksUser != "" {
		auth := url.UserPassword(cfg.SocksUser, cfg.SocksPass).String()
		return fmt.Sprintf("socks5://%s@%s:%d#%s", auth, addr, cfg.SocksPort, url.QueryEscape(tag)), nil
	}

	return fmt.Sprintf("socks5://%s:%d#%s", addr, cfg.SocksPort, url.QueryEscape(tag)), nil
}

// VLESSGenerator generates vless:// links (including VLESS Reality)
type VLESSGenerator struct{}

func (g *VLESSGenerator) Protocol() string { return "vless" }

func (g *VLESSGenerator) IsSupported(cfg RuntimeConfig) bool {
	return cfg.VlessEnabled && cfg.VlessUUID != "" && cfg.VlessPort > 0
}

func (g *VLESSGenerator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("VLESS is not active in current Xray runtime configuration")
	}

	addr := cfg.VlessAddress
	if addr == "" {
		addr = "127.0.0.1"
	}

	tag := fmt.Sprintf("super-proxy-%s-%s", node.Country, node.ID)

	q := url.Values{}
	vType := cfg.VlessType
	if vType == "" {
		vType = "tcp"
	}
	q.Set("type", vType)

	if cfg.VlessSecurity == "reality" {
		q.Set("security", "reality")
		if cfg.VlessSNI != "" {
			q.Set("sni", cfg.VlessSNI)
		}
		if cfg.VlessFingerprint != "" {
			q.Set("fp", cfg.VlessFingerprint)
		} else {
			q.Set("fp", "chrome")
		}
		if cfg.VlessPublicKey != "" {
			q.Set("pbk", cfg.VlessPublicKey)
		}
		if cfg.VlessShortID != "" {
			q.Set("sid", cfg.VlessShortID)
		}
		if cfg.VlessFlow != "" {
			q.Set("flow", cfg.VlessFlow)
		}
		q.Set("encryption", "none")
	} else if cfg.VlessSecurity == "tls" {
		q.Set("security", "tls")
		if cfg.VlessSNI != "" {
			q.Set("sni", cfg.VlessSNI)
		}
		if cfg.VlessFlow != "" {
			q.Set("flow", cfg.VlessFlow)
		}
		q.Set("encryption", "none")
	} else {
		q.Set("security", "none")
		q.Set("encryption", "none")
	}

	link := fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		cfg.VlessUUID,
		addr,
		cfg.VlessPort,
		q.Encode(),
		url.QueryEscape(tag),
	)

	return link, nil
}

// VMessGenerator generates vmess:// base64 json links
type VMessGenerator struct{}

func (g *VMessGenerator) Protocol() string { return "vmess" }

func (g *VMessGenerator) IsSupported(cfg RuntimeConfig) bool {
	for _, p := range cfg.SupportedProtocols {
		if strings.EqualFold(p, "vmess") {
			return true
		}
	}
	return false
}

func (g *VMessGenerator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("VMess protocol is not enabled in backend Xray runtime")
	}
	tag := fmt.Sprintf("super-proxy-%s-%s", node.Country, node.ID)
	vmessData := map[string]interface{}{
		"v":    "2",
		"ps":   tag,
		"add":  cfg.VlessAddress,
		"port": strconv.Itoa(cfg.VlessPort),
		"id":   cfg.VlessUUID,
		"aid":  "0",
		"scy":  "auto",
		"net":  "tcp",
		"type": "none",
		"host": "",
		"path": "",
		"tls":  "",
	}
	bytes, _ := json.Marshal(vmessData)
	return "vmess://" + base64.StdEncoding.EncodeToString(bytes), nil
}

// TrojanGenerator generates trojan:// links
type TrojanGenerator struct{}

func (g *TrojanGenerator) Protocol() string { return "trojan" }

func (g *TrojanGenerator) IsSupported(cfg RuntimeConfig) bool {
	for _, p := range cfg.SupportedProtocols {
		if strings.EqualFold(p, "trojan") {
			return true
		}
	}
	return false
}

func (g *TrojanGenerator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("Trojan protocol is not enabled in backend Xray runtime")
	}
	return "", errors.New("Trojan configuration not found in backend")
}

// ShadowsocksGenerator generates ss:// SIP002 links
type ShadowsocksGenerator struct{}

func (g *ShadowsocksGenerator) Protocol() string { return "shadowsocks" }

func (g *ShadowsocksGenerator) IsSupported(cfg RuntimeConfig) bool {
	for _, p := range cfg.SupportedProtocols {
		if strings.EqualFold(p, "shadowsocks") || strings.EqualFold(p, "ss") {
			return true
		}
	}
	return false
}

func (g *ShadowsocksGenerator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("Shadowsocks protocol is not enabled in backend Xray runtime")
	}
	return "", errors.New("Shadowsocks configuration not found in backend")
}

// HTTPGenerator generates http:// proxy links
type HTTPGenerator struct{}

func (g *HTTPGenerator) Protocol() string { return "http" }

func (g *HTTPGenerator) IsSupported(cfg RuntimeConfig) bool {
	for _, p := range cfg.SupportedProtocols {
		if strings.EqualFold(p, "http") {
			return true
		}
	}
	return false
}

func (g *HTTPGenerator) Generate(node NodeInfo, cfg RuntimeConfig) (string, error) {
	if !g.IsSupported(cfg) {
		return "", errors.New("HTTP proxy protocol is not enabled in backend Xray runtime")
	}
	return "", errors.New("HTTP proxy configuration not found in backend")
}
