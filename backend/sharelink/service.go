package sharelink

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Service struct {
	generators map[string]ShareLinkGenerator
}

func NewService() *Service {
	s := &Service{
		generators: make(map[string]ShareLinkGenerator),
	}
	s.Register(&SOCKS5Generator{})
	s.Register(&VLESSGenerator{})
	s.Register(&VMessGenerator{})
	s.Register(&TrojanGenerator{})
	s.Register(&ShadowsocksGenerator{})
	s.Register(&HTTPGenerator{})
	return s
}

func (s *Service) Register(g ShareLinkGenerator) {
	s.generators[strings.ToLower(g.Protocol())] = g
}

func (s *Service) GetSupportedProtocols(cfg RuntimeConfig) []string {
	var supported []string
	for proto, g := range s.generators {
		if g.IsSupported(cfg) {
			supported = append(supported, proto)
		}
	}
	return supported
}

func (s *Service) Generate(node NodeInfo, protocol string, cfg RuntimeConfig) (*ShareLinkResult, error) {
	proto := strings.ToLower(protocol)
	g, exists := s.generators[proto]
	if !exists {
		return &ShareLinkResult{
			Protocol:  protocol,
			Supported: false,
			NodeID:    node.ID,
			Country:   node.Country,
			Error:     fmt.Sprintf("unknown protocol: %s", protocol),
		}, errors.New("unknown protocol")
	}

	if !g.IsSupported(cfg) {
		return &ShareLinkResult{
			Protocol:    protocol,
			Supported:   false,
			NodeID:      node.ID,
			Country:     node.Country,
			Description: fmt.Sprintf("%s is NOT SUPPORTED by backend runtime configuration", strings.ToUpper(protocol)),
			Error:       "protocol not supported in active runtime",
		}, nil
	}

	uri, err := g.Generate(node, cfg)
	if err != nil {
		return &ShareLinkResult{
			Protocol:  protocol,
			Supported: true,
			NodeID:    node.ID,
			Country:   node.Country,
			Error:     err.Error(),
		}, err
	}

	// Strict Security Audit on Generated URI
	if err := ValidateGeneratedURI(uri); err != nil {
		return nil, fmt.Errorf("security violation in generated sharelink: %w", err)
	}

	return &ShareLinkResult{
		Protocol:    protocol,
		Supported:   true,
		URI:         uri,
		NodeID:      node.ID,
		Country:     node.Country,
		Description: fmt.Sprintf("%s proxy for %s (%s)", strings.ToUpper(protocol), node.Country, node.IP),
	}, nil
}

func (s *Service) BatchGenerate(nodes []NodeInfo, protocols []string, cfg RuntimeConfig) []*ShareLinkResult {
	var results []*ShareLinkResult
	for _, node := range nodes {
		for _, proto := range protocols {
			res, _ := s.Generate(node, proto, cfg)
			if res != nil {
				results = append(results, res)
			}
		}
	}
	return results
}

// ValidateGeneratedURI audits output for leaked secrets, private keys, or VPN credentials
func ValidateGeneratedURI(rawURI string) error {
	if rawURI == "" {
		return errors.New("empty URI")
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return fmt.Errorf("malformed URI: %w", err)
	}

	lower := strings.ToLower(rawURI)

	// Critical check: Ensure no private key markers or VPN Gate configs are present
	leaks := []string{
		"private_key",
		"privatekey",
		"-----begin",
		"cert_pass",
		"openvpn",
		"client-key",
		"pkcs12",
	}
	for _, leak := range leaks {
		if strings.Contains(lower, leak) {
			return fmt.Errorf("detected forbidden sensitive marker %q in URI", leak)
		}
	}

	// Verify required scheme
	if u.Scheme == "" {
		return errors.New("URI missing scheme")
	}

	return nil
}
