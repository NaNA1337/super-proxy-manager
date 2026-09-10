package sharelink

type NodeInfo struct {
	ID          string `json:"id"`
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryLong string `json:"country_long"`
	HostName    string `json:"hostname"`
	Score       int    `json:"score"`
	Status      string `json:"status"`
}

type RuntimeConfig struct {
	SupportedProtocols []string          `json:"supported_protocols"`
	SocksAddress       string            `json:"socks_address"`
	SocksPort          int               `json:"socks_port"`
	SocksUser          string            `json:"socks_user,omitempty"`
	SocksPass          string            `json:"socks_pass,omitempty"`
	VlessEnabled       bool              `json:"vless_enabled"`
	VlessAddress       string            `json:"vless_address,omitempty"`
	VlessPort          int               `json:"vless_port,omitempty"`
	VlessUUID          string            `json:"vless_uuid,omitempty"`
	VlessSecurity      string            `json:"vless_security,omitempty"` // "reality" or "none"
	VlessSNI           string            `json:"vless_sni,omitempty"`
	VlessFingerprint   string            `json:"vless_fingerprint,omitempty"`
	VlessPublicKey     string            `json:"vless_public_key,omitempty"`
	VlessShortID       string            `json:"vless_short_id,omitempty"`
	VlessFlow          string            `json:"vless_flow,omitempty"`
	VlessType          string            `json:"vless_type,omitempty"` // "tcp"
}

type ShareLinkResult struct {
	Protocol    string `json:"protocol"`
	Supported   bool   `json:"supported"`
	URI         string `json:"uri,omitempty"`
	NodeID      string `json:"node_id"`
	Country     string `json:"country"`
	Description string `json:"description"`
	Error       string `json:"error,omitempty"`
}

type ShareLinkGenerator interface {
	Protocol() string
	IsSupported(cfg RuntimeConfig) bool
	Generate(node NodeInfo, cfg RuntimeConfig) (string, error)
}
