package host

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var privateHostsAllowed atomic.Bool

var (
	// Blocked IPv4 CIDR ranges
	blockedIPv4CIDRs = []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918 Private
		"172.16.0.0/12",  // RFC1918 Private
		"192.168.0.0/16", // RFC1918 Private
		"169.254.0.0/16", // Link-local / APIPA
		"100.64.0.0/10",  // Carrier Grade NAT / Shared
		"0.0.0.0/8",      // Current network
	}

	// Blocked IPv6 CIDR ranges
	blockedIPv6CIDRs = []string{
		"::1/128",   // IPv6 loopback
		"fc00::/7",  // IPv6 unique local (ULA)
		"fe80::/10", // IPv6 link-local
	}

	// Exact metadata IPs to explicitly block
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

	parsedIPv4CIDRs []*net.IPNet
	parsedIPv6CIDRs []*net.IPNet
)

func init() {
	for _, cidr := range blockedIPv4CIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedIPv4CIDRs = append(parsedIPv4CIDRs, ipNet)
		}
	}
	for _, cidr := range blockedIPv6CIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedIPv6CIDRs = append(parsedIPv6CIDRs, ipNet)
		}
	}
}

// SetPrivateHostsAllowed configures whether private/loopback Agent targets are allowed.
// The Manager command uses this for the -allow-private-hosts flag.
func SetPrivateHostsAllowed(allowed bool) {
	privateHostsAllowed.Store(allowed)
}

// IsPrivateHostsAllowed checks the command policy and backward-compatible environment settings.
func IsPrivateHostsAllowed() bool {
	return privateHostsAllowed.Load() || os.Getenv("ALLOW_PRIVATE_HOSTS") == "true" || os.Getenv("SPM_ALLOW_PRIVATE_HOSTS") == "true"
}

// NormalizeIP unmaps IPv4-mapped IPv6 addresses to pure IPv4
func NormalizeIP(ip net.IP) net.IP {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4
	}
	return ip
}

// IsProhibitedIP checks if an IP is in private, link-local, loopback, or metadata CIDRs
func IsProhibitedIP(ip net.IP, allowPrivate bool) error {
	normIP := NormalizeIP(ip)

	// Check metadata exact IPs first (always prohibited, even in test environments)
	for _, blocked := range blockedIPs {
		if normIP.Equal(net.ParseIP(blocked)) {
			return fmt.Errorf("access to cloud metadata IP %q is prohibited", normIP.String())
		}
	}

	// Link-local is always prohibited
	if normIP.IsLinkLocalUnicast() || normIP.IsLinkLocalMulticast() {
		return fmt.Errorf("access to link-local IP %q is prohibited", normIP.String())
	}

	// Multicast is always prohibited
	if normIP.IsMulticast() {
		return fmt.Errorf("access to multicast IP %q is prohibited", normIP.String())
	}

	if allowPrivate {
		return nil
	}

	// Loopback check
	if normIP.IsLoopback() {
		return fmt.Errorf("access to loopback IP %q is prohibited by SSRF policy", normIP.String())
	}

	// CIDR blocks check: if IPv4, check against parsedIPv4CIDRs
	if ip4 := normIP.To4(); ip4 != nil {
		for _, cidr := range parsedIPv4CIDRs {
			if cidr.Contains(ip4) {
				return fmt.Errorf("access to private or restricted network %q (%s) is prohibited", normIP.String(), cidr.String())
			}
		}
	} else {
		// IPv6
		for _, cidr := range parsedIPv6CIDRs {
			if cidr.Contains(normIP) {
				return fmt.Errorf("access to private or restricted network %q (%s) is prohibited", normIP.String(), cidr.String())
			}
		}
	}

	return nil
}

// ValidateAgentURL verifies that an agent API URL is safe from SSRF attacks
func ValidateAgentURL(rawURL string) error {
	return ValidateAgentURLWithPolicy(rawURL, IsPrivateHostsAllowed())
}

// ValidateAgentURLWithPolicy verifies an agent API URL with explicit allowPrivate flag
func ValidateAgentURLWithPolicy(rawURL string, allowPrivate bool) error {
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

	// 2. Prohibit userinfo (e.g. https://user@127.0.0.1)
	if u.User != nil {
		return errors.New("embedded credentials / userinfo in agent URL are prohibited")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return errors.New("URL hostname cannot be empty")
	}

	// 3. Check blocked metadata hostnames
	lowerHostname := strings.ToLower(hostname)
	for _, blocked := range blockedHostnames {
		if lowerHostname == blocked || strings.HasSuffix(lowerHostname, "."+blocked) {
			return fmt.Errorf("access to cloud metadata endpoint %q is prohibited", hostname)
		}
	}

	// 4. Localhost check
	if lowerHostname == "localhost" {
		if !allowPrivate {
			return errors.New("access to localhost is prohibited by SSRF policy")
		}
	}

	// 5. Port check
	portStr := u.Port()
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port number %q", portStr)
		}
	}

	// 6. Direct IP check
	ip := net.ParseIP(hostname)
	if ip != nil {
		if err := IsProhibitedIP(ip, allowPrivate); err != nil {
			return err
		}
	}

	return nil
}

// NewSafeTransport creates an http.Transport with DNS resolution and connection-time SSRF guards
func NewSafeTransport(tlsConfig *tls.Config, allowPrivate bool) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}

			// Prohibit localhost hostname unless explicitly allowed
			if strings.EqualFold(host, "localhost") && !allowPrivate {
				return nil, errors.New("access to localhost is prohibited by SSRF policy")
			}

			// Resolve host to inspect final IPs (prevents DNS rebinding)
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed for %q: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses resolved for %q", host)
			}

			// Validate all resolved IPs
			var selectedIP net.IP
			for _, ipAddr := range ips {
				if err := IsProhibitedIP(ipAddr.IP, allowPrivate); err != nil {
					return nil, fmt.Errorf("SSRF protection: resolved IP %s for %s is prohibited: %w", ipAddr.IP.String(), host, err)
				}
				if selectedIP == nil {
					selectedIP = ipAddr.IP
				}
			}

			// Dial the verified IP directly
			target := net.JoinHostPort(selectedIP.String(), port)
			return dialer.DialContext(ctx, network, target)
		},
	}
}

// SafeCheckRedirect returns a redirect check function enforcing SSRF policy on redirects
func SafeCheckRedirect(allowPrivate bool) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if err := ValidateAgentURLWithPolicy(req.URL.String(), allowPrivate); err != nil {
			return fmt.Errorf("redirect blocked by SSRF policy: %w", err)
		}
		return nil
	}
}
