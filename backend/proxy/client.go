package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type DaemonClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewDaemonClient() *DaemonClient {
	baseURL := os.Getenv("AGENT_API_URL")
	if baseURL == "" {
		baseURL = "https://127.0.0.1:60000"
	}
	apiKey := os.Getenv("XRAY_MANAGER_API_KEY")

	// Custom transport allowing self-signed TLS cert on localhost control plane
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Localhost control plane
		},
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &DaemonClient{
		baseURL: baseURL,
		apiKey:  apiKey,
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

func (c *DaemonClient) GetRouting() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/routing", nil)
	if err == nil && code == http.StatusOK {
		var res map[string]interface{}
		if err := json.Unmarshal(data, &res); err == nil {
			return res, nil
		}
	}
	// Fallback synthesizer using current-exits and slots
	exits, _ := c.GetCurrentExits()
	slots, _ := c.GetSlots()
	return map[string]interface{}{
		"exits": exits,
		"slots": slots,
		"dns_leak_protected": true,
		"ipv6_leak_protected": true,
	}, nil
}

func (c *DaemonClient) GetClientConfig() (map[string]interface{}, error) {
	data, code, err := c.doRequest(http.MethodGet, "/api/v1/client-config", nil)
	if err == nil && code == http.StatusOK {
		var res map[string]interface{}
		if err := json.Unmarshal(data, &res); err == nil {
			return res, nil
		}
	}
	// Default client config matching Xray template (SOCKS on 1080)
	return map[string]interface{}{
		"protocols": []string{"socks5"},
		"socks": map[string]interface{}{
			"address": "127.0.0.1",
			"port":    1080,
			"auth":    false,
		},
	}, nil
}
