package sharelink

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ClientProfile struct {
	HostID           string                 `json:"host_id"`
	HostName         string                 `json:"host_name"`
	NodeID           string                 `json:"node_id"`
	Name             string                 `json:"name"`
	Protocol         string                 `json:"protocol"`
	URI              string                 `json:"uri"`
	XrayConfig       map[string]interface{} `json:"xray_config,omitempty"`
	SingBoxConfig    map[string]interface{} `json:"singbox_config,omitempty"`
	ClashConfig      string                 `json:"clash_config,omitempty"`
	SupportedClients []string               `json:"supported_clients"`
}

// GenerateClientProfile produces multi-format client configurations based on real runtime config
func GenerateClientProfile(hostID, hostName string, node NodeInfo, protocol string, cfg RuntimeConfig) (*ClientProfile, error) {
	tag := fmt.Sprintf("super-proxy-%s-%s", node.Country, node.ID)
	if node.Country == "" {
		tag = fmt.Sprintf("super-proxy-%s", node.ID)
	}

	profile := &ClientProfile{
		HostID:   hostID,
		HostName: hostName,
		NodeID:   node.ID,
		Name:     tag,
		Protocol: protocol,
	}

	switch strings.ToLower(protocol) {
	case "vless":
		if !cfg.VlessEnabled || cfg.VlessUUID == "" || cfg.VlessPort <= 0 {
			return nil, fmt.Errorf("VLESS is not active in current Xray runtime configuration")
		}

		gen := &VLESSGenerator{}
		uri, err := gen.Generate(node, cfg)
		if err != nil {
			return nil, err
		}
		profile.URI = uri
		profile.SupportedClients = []string{"v2rayN", "v2rayNG", "Clash Meta", "Mihomo", "sing-box", "Xray-core", "NekoBox", "Shadowrocket"}

		addr := cfg.VlessAddress
		if addr == "" {
			addr = "127.0.0.1"
		}

		// 1. Clash Meta / Mihomo (YAML)
		var clashLines []string
		clashLines = append(clashLines, fmt.Sprintf("  - name: %q", tag))
		clashLines = append(clashLines, "    type: vless")
		clashLines = append(clashLines, fmt.Sprintf("    server: %s", addr))
		clashLines = append(clashLines, fmt.Sprintf("    port: %d", cfg.VlessPort))
		clashLines = append(clashLines, fmt.Sprintf("    uuid: %s", cfg.VlessUUID))
		clashLines = append(clashLines, "    network: tcp")
		clashLines = append(clashLines, "    udp: true")
		if cfg.VlessFlow != "" {
			clashLines = append(clashLines, fmt.Sprintf("    flow: %s", cfg.VlessFlow))
		}
		if cfg.VlessSecurity == "reality" {
			clashLines = append(clashLines, "    tls: true")
			if cfg.VlessSNI != "" {
				clashLines = append(clashLines, fmt.Sprintf("    servername: %s", cfg.VlessSNI))
			}
			fp := cfg.VlessFingerprint
			if fp == "" {
				fp = "chrome"
			}
			clashLines = append(clashLines, fmt.Sprintf("    client-fingerprint: %s", fp))
			clashLines = append(clashLines, "    reality-opts:")
			if cfg.VlessPublicKey != "" {
				clashLines = append(clashLines, fmt.Sprintf("      public-key: %s", cfg.VlessPublicKey))
			}
			if cfg.VlessShortID != "" {
				clashLines = append(clashLines, fmt.Sprintf("      short-id: %s", cfg.VlessShortID))
			}
		} else if cfg.VlessSecurity == "tls" {
			clashLines = append(clashLines, "    tls: true")
			if cfg.VlessSNI != "" {
				clashLines = append(clashLines, fmt.Sprintf("    servername: %s", cfg.VlessSNI))
			}
		}
		profile.ClashConfig = strings.Join(clashLines, "\n")

		// 2. sing-box (JSON)
		singboxOutbound := map[string]interface{}{
			"type":        "vless",
			"tag":         tag,
			"server":      addr,
			"server_port": cfg.VlessPort,
			"uuid":        cfg.VlessUUID,
			"network":     "tcp",
		}
		if cfg.VlessFlow != "" {
			singboxOutbound["flow"] = cfg.VlessFlow
		}
		if cfg.VlessSecurity == "reality" {
			tlsBlock := map[string]interface{}{
				"enabled":     true,
				"server_name": cfg.VlessSNI,
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": "chrome",
				},
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": cfg.VlessPublicKey,
					"short_id":   cfg.VlessShortID,
				},
			}
			singboxOutbound["tls"] = tlsBlock
		} else if cfg.VlessSecurity == "tls" {
			singboxOutbound["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": cfg.VlessSNI,
			}
		}
		profile.SingBoxConfig = singboxOutbound

		// 3. Xray-core (JSON)
		xrayStreamSettings := map[string]interface{}{
			"network":  "tcp",
			"security": cfg.VlessSecurity,
		}
		if cfg.VlessSecurity == "reality" {
			xrayStreamSettings["realitySettings"] = map[string]interface{}{
				"serverName":  cfg.VlessSNI,
				"fingerprint": "chrome",
				"show":        false,
				"publicKey":   cfg.VlessPublicKey,
				"shortId":     cfg.VlessShortID,
				"spiderX":     "",
			}
		} else if cfg.VlessSecurity == "tls" {
			xrayStreamSettings["tlsSettings"] = map[string]interface{}{
				"serverName": cfg.VlessSNI,
			}
		}
		userItem := map[string]interface{}{
			"id":         cfg.VlessUUID,
			"encryption": "none",
		}
		if cfg.VlessFlow != "" {
			userItem["flow"] = cfg.VlessFlow
		}
		profile.XrayConfig = map[string]interface{}{
			"tag":      tag,
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []map[string]interface{}{
					{
						"address": addr,
						"port":    cfg.VlessPort,
						"users":   []map[string]interface{}{userItem},
					},
				},
			},
			"streamSettings": xrayStreamSettings,
		}

	case "socks5":
		if cfg.SocksPort <= 0 {
			return nil, fmt.Errorf("SOCKS5 is not configured in backend runtime")
		}
		gen := &SOCKS5Generator{}
		uri, err := gen.Generate(node, cfg)
		if err != nil {
			return nil, err
		}
		profile.URI = uri
		profile.SupportedClients = []string{"Clash Meta", "Mihomo", "sing-box", "Xray-core", "SOCKS5 Client", "Telegram"}

		addr := cfg.SocksAddress
		if addr == "" || addr == "0.0.0.0" {
			addr = "127.0.0.1"
		}

		profile.ClashConfig = fmt.Sprintf("  - name: %q\n    type: socks5\n    server: %s\n    port: %d", tag, addr, cfg.SocksPort)
		profile.SingBoxConfig = map[string]interface{}{
			"type":        "socks",
			"tag":         tag,
			"server":      addr,
			"server_port": cfg.SocksPort,
		}
		profile.XrayConfig = map[string]interface{}{
			"tag":      tag,
			"protocol": "socks",
			"settings": map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"address": addr,
						"port":    cfg.SocksPort,
					},
				},
			},
		}

	default:
		return nil, fmt.Errorf("protocol %q is not currently enabled in backend runtime", protocol)
	}

	return profile, nil
}

// GenerateStructuredAllText produces a structured, readable plain-text compilation of all client links
func GenerateStructuredAllText(hostName, hostAddress string, profiles []*ClientProfile) string {
	var b strings.Builder
	b.WriteString("================================================================================\n")
	b.WriteString("SUPER-PROXY CLIENT LINKS EXPORT\n")
	b.WriteString(fmt.Sprintf("Host: %s (%s)\n", hostName, hostAddress))
	b.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().UTC().Format(time.RFC1123)))
	b.WriteString(fmt.Sprintf("Total Profiles: %d\n", len(profiles)))
	b.WriteString("================================================================================\n\n")

	for i, p := range profiles {
		b.WriteString(fmt.Sprintf("[%d] %s (%s)\n", i+1, p.Name, strings.ToUpper(p.Protocol)))
		b.WriteString(fmt.Sprintf("URI:\n%s\n\n", p.URI))

		if p.ClashConfig != "" {
			b.WriteString(fmt.Sprintf("Clash Meta / Mihomo (YAML):\n%s\n\n", p.ClashConfig))
		}

		if p.SingBoxConfig != nil {
			sbJSON, _ := json.MarshalIndent(p.SingBoxConfig, "", "  ")
			b.WriteString(fmt.Sprintf("sing-box Outbound (JSON):\n%s\n\n", string(sbJSON)))
		}

		if p.XrayConfig != nil {
			xrayJSON, _ := json.MarshalIndent(p.XrayConfig, "", "  ")
			b.WriteString(fmt.Sprintf("Xray-core Outbound (JSON):\n%s\n\n", string(xrayJSON)))
		}

		b.WriteString("--------------------------------------------------------------------------------\n\n")
	}

	return b.String()
}
