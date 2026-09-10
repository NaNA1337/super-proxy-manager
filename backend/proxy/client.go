package proxy

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// CanonicalProfile represents a single client configuration profile provided by super-proxy
type CanonicalProfile struct {
	ID          string `json:"id"`                    // e.g. "vless", "clash", "sing-box", "xray", "subscription"
	Name        string `json:"name"`                  // e.g. "VLESS URI", "Clash Meta", "sing-box", "Xray"
	Format      string `json:"format"`                // "uri", "yaml", "json", "text"
	Filename    string `json:"filename"`              // e.g. "vless.txt", "clash-meta.yaml", "sing-box.json"
	MimeType    string `json:"mime_type"`             // e.g. "text/plain", "application/x-yaml", "application/json"
	Content     string `json:"content"`               // Raw configuration string
	Description string `json:"description,omitempty"` // e.g. "VLESS Reality / Vision"
	CanQR       bool   `json:"can_qr,omitempty"`      // true if QR code generation is suitable
}

// NodeClientConfig represents all canonical profiles for a specific node on a host
type NodeClientConfig struct {
	Available       bool               `json:"available"`
	Error           string             `json:"error,omitempty"`
	HostID          string             `json:"host_id,omitempty"`
	HostName        string             `json:"host_name,omitempty"`
	NodeID          string             `json:"node_id"`
	NodeIP          string             `json:"node_ip,omitempty"`
	Country         string             `json:"country,omitempty"`
	EndpointAddress string             `json:"endpoint_address,omitempty"`
	EndpointPort    int                `json:"endpoint_port,omitempty"`
	Protocol        string             `json:"protocol,omitempty"`
	Transport       string             `json:"transport,omitempty"`
	Flow            string             `json:"flow,omitempty"`
	UpdatedAt       string             `json:"updated_at,omitempty"`
	Profiles        []CanonicalProfile `json:"profiles"`
}

// AllClientConfigResponse is the canonical response from super-proxy GET /api/v1/client-config/all
type AllClientConfigResponse struct {
	Available bool               `json:"available"`
	Error     string             `json:"error,omitempty"`
	Nodes     []NodeClientConfig `json:"nodes,omitempty"`
	Profiles  []CanonicalProfile `json:"profiles,omitempty"`
}

type DaemonClient struct {
	baseURL           string
	apiKey            string
	pinnedFingerprint string
	httpClient        *http.Client
}

// NormalizeFingerprint standardizes fingerprint string to lowercase hex
func NormalizeFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	fp = strings.TrimPrefix(fp, "SHA256:")
	fp = strings.TrimPrefix(fp, "sha256:")
	fp = strings.ReplaceAll(fp, ":", "")
	fp = strings.ReplaceAll(fp, " ", "")
	return strings.ToLower(fp)
}

// FormatFingerprint converts raw DER certificate bytes into "SHA256:XX:XX..." representation
func FormatFingerprint(rawDER []byte) string {
	sum := sha256.Sum256(rawDER)
	var parts []string
	for _, b := range sum {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return "SHA256:" + strings.Join(parts, ":")
}

// ProbeTLS connects to the target agent URL via TLS and extracts the server leaf certificate fingerprint
func ProbeTLS(agentURL string) (string, error) {
	u, err := url.Parse(agentURL)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", nil
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}
	targetAddr := net.JoinHostPort(host, port)

	conf := &tls.Config{
		InsecureSkipVerify: true, // Used only to inspect the peer certificate for fingerprinting
		ServerName:         host,
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", targetAddr, conf)
	if err != nil {
		return "", fmt.Errorf("tls probe failed: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", errors.New("no peer certificates presented by remote host")
	}

	leaf := state.PeerCertificates[0]
	return FormatFingerprint(leaf.Raw), nil
}

func NewDaemonClient() *DaemonClient {
	baseURL := os.Getenv("AGENT_API_URL")
	if baseURL == "" {
		baseURL = "https://127.0.0.1:60000"
	}
	apiKey := os.Getenv("XRAY_MANAGER_API_KEY")
	return NewClient(baseURL, apiKey)
}

func NewClient(baseURL, apiKey string) *DaemonClient {
	return NewClientWithFingerprint(baseURL, apiKey, "")
}

func NewClientWithFingerprint(baseURL, apiKey, pinnedFingerprint string) *DaemonClient {
	normPinned := NormalizeFingerprint(pinnedFingerprint)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("remote host presented no TLS certificates")
			}
			if normPinned != "" {
				leafHash := sha256.Sum256(rawCerts[0])
				leafHex := hex.EncodeToString(leafHash[:])
				if !strings.EqualFold(leafHex, normPinned) {
					return fmt.Errorf("TLS certificate fingerprint mismatch: expected %s, got SHA256:%s", pinnedFingerprint, strings.ToUpper(leafHex))
				}
			}
			return nil
		},
	}

	tr := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &DaemonClient{
		baseURL:           baseURL,
		apiKey:            apiKey,
		pinnedFingerprint: pinnedFingerprint,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
	}
}

func (c *DaemonClient) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *DaemonClient) SetBaseURL(u string) {
	c.baseURL = u
}

func (c *DaemonClient) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("daemon connection error: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respData, resp.StatusCode, nil
}

func (c *DaemonClient) GetStatus() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/status", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetSystem() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/system", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetCurrentExits() ([]map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/current-exits", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res []map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetSlots() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/slots", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetPool() (map[string]int, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/pool", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]int
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetPoolQualified() ([]map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/pool/qualified", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res []map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetNodes(country, status, search string, limit, offset int) (map[string]interface{}, error) {
	params := url.Values{}
	if country != "" {
		params.Set("country", country)
	}
	if status != "" {
		params.Set("status", status)
	}
	if search != "" {
		params.Set("search", search)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/api/v1/nodes"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	data, code, err := c.doRequest(http.MethodGet, path, nil)
	if err == nil && code == http.StatusOK {
		var res map[string]interface{}
		if err := json.Unmarshal(data, &res); err == nil {
			return res, nil
		}
	}

	// Fallback to pool/qualified if daemon doesn't have /api/v1/nodes yet
	qualified, qErr := c.GetPoolQualified()
	if qErr != nil {
		return nil, qErr
	}
	return map[string]interface{}{
		"total": len(qualified),
		"nodes": qualified,
	}, nil
}

func (c *DaemonClient) GetNodeDetails(nodeID string) (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/nodes/"+url.PathEscape(nodeID), nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) SwitchSlot(slot int, targetNodeID string) (map[string]interface{}, int, error) {
	reqBody := map[string]string{
		"node_id": targetNodeID,
	}
	data, code, err := c.doRequest(http.MethodPost, fmt.Sprintf("/api/v1/slots/%d/switch", slot), reqBody)
	if err != nil {
		return nil, code, err
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return map[string]interface{}{"raw": string(data)}, code, nil
	}
	return res, code, nil
}

func (c *DaemonClient) GetOperation(operationID string) (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/operations/"+url.PathEscape(operationID), nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *DaemonClient) GetMetrics() (string, error) {
	data, code, err := c.doRequest(http.MethodGet, "/metrics", nil)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	return string(data), nil
}

// GetRouting retrieves routing information directly from daemon. No fake fallback synthesized.
func (c *DaemonClient) GetRouting() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/routing", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetClientConfig returns inbound configuration from daemon. Strictly returns error on failure (no fake 127.0.0.1 fallback).
func (c *DaemonClient) GetClientConfig() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/client-config", nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAllClientConfig fetches canonical multi-profile configuration from super-proxy GET /api/v1/client-config/all
func (c *DaemonClient) GetAllClientConfig(nodeID string) (*AllClientConfigResponse, error) {
	path := "/api/v1/client-config/all"
	if nodeID != "" {
		path += "?node_id=" + url.QueryEscape(nodeID)
	}

	data, code, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return &AllClientConfigResponse{
			Available: false,
			Error:     fmt.Sprintf("super-proxy daemon connection error: %v", err),
		}, err
	}

	// Handle runtime endpoint unavailable from daemon
	if code == http.StatusServiceUnavailable || code == http.StatusNotFound {
		errMsg := "runtime endpoint unavailable: Xray unavailable or stopped"
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &errResp) == nil {
			if errResp.Error != "" {
				errMsg = errResp.Error
			} else if errResp.Message != "" {
				errMsg = errResp.Message
			}
		}
		return &AllClientConfigResponse{
			Available: false,
			Error:     errMsg,
		}, nil
	}

	if code != http.StatusOK {
		return &AllClientConfigResponse{
			Available: false,
			Error:     fmt.Sprintf("daemon returned status %d: %s", code, string(data)),
		}, fmt.Errorf("daemon returned status %d: %s", code, string(data))
	}

	// Parse JSON
	var resp AllClientConfigResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return &AllClientConfigResponse{
			Available: false,
			Error:     fmt.Sprintf("invalid JSON from daemon: %v", err),
		}, err
	}

	// If response returned top-level profiles and nodeID was queried, synthesize Node struct if needed
	if len(resp.Profiles) > 0 && len(resp.Nodes) == 0 && nodeID != "" {
		resp.Nodes = []NodeClientConfig{
			{
				Available: resp.Available,
				Error:     resp.Error,
				NodeID:    nodeID,
				Profiles:  resp.Profiles,
			},
		}
	}

	return &resp, nil
}
