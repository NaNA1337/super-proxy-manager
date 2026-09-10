package host

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var (
	// Metadata IP addresses to explicitly block
	blockedIPs = []string{
		"169.254.169.254", // AWS/GCP/Azure link-local metadata
		"100.100.100.200", // Alibaba Cloud metadata
		"169.254.169.253", // AWS DNS link-local
	}

	// Metadata hostnames to explicitly block
	blockedHostnames = []string{
		"metadata.google.internal",
		"metadata.internal",
		"instance-data",
	}
)

// ValidateAgentURL verifies that an agent API URL is safe from SSRF attacks
func ValidateAgentURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("agent API URL cannot be empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// 1. Strict scheme check: http or https only
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported protocol scheme %q: only http and https are permitted", scheme)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return errors.New("URL hostname cannot be empty")
	}

	// 2. Check blocked hostnames
	lowerHostname := strings.ToLower(hostname)
	for _, blocked := range blockedHostnames {
		if lowerHostname == blocked || strings.HasSuffix(lowerHostname, "."+blocked) {
			return fmt.Errorf("access to cloud metadata endpoint %q is prohibited", hostname)
		}
	}

	// 3. Port check
	portStr := u.Port()
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port number %q", portStr)
		}
	}

	// 4. IP check
	ip := net.ParseIP(hostname)
	if ip != nil {
		// Check exact blocked IPs
		for _, blocked := range blockedIPs {
			if ip.Equal(net.ParseIP(blocked)) {
				return fmt.Errorf("access to link-local metadata IP %q is prohibited", ip.String())
			}
		}

		// Block Link-Local unicast (169.254.0.0/16, fe80::/10)
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("access to link-local IP %q is prohibited", ip.String())
		}

		// Block Multicast
		if ip.IsMulticast() {
			return fmt.Errorf("access to multicast IP %q is prohibited", ip.String())
		}

		// Loopback (127.0.0.1, ::1) is permitted for managing the local co-located super-proxy daemon
		if ip.IsLoopback() {
			return nil
		}
	} else if lowerHostname == "localhost" {
		// localhost is permitted for local co-located daemon
		return nil
	}

	return nil
}
