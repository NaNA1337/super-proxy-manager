export type Role = 'admin' | 'readonly';

export interface UserSession {
  authenticated: boolean;
  must_change_password?: boolean;
  username?: string;
  role?: Role;
  csrf_token?: string;
  expires_at?: string;
}

export interface Host {
  id: string;
  name: string;
  address: string;
  agent_url: string;
  enabled: boolean;
  status: string; // 'online' | 'healthy' | 'degraded' | 'offline' | 'unknown' | 'incompatible'
  last_seen?: string;
  version: string;
  region: string;
  tls_fingerprint?: string;
  is_default: boolean;
  active_slots?: number;
  latency_ms?: number;
  created_at: string;
  updated_at: string;
}

export interface TestConnectionResult {
  success: boolean;
  status: string;
  version?: string;
  region?: string;
  latency_ms?: number;
  tls_fingerprint?: string;
  detected_fingerprint?: string;
  error?: string;
}

export interface CanonicalProfile {
  id: string;
  name: string;
  format: 'uri' | 'yaml' | 'json' | 'text' | string;
  filename: string;
  mime_type: string;
  content: string;
  description?: string;
  can_qr?: boolean;
}

export interface NodeClientConfig {
  available: boolean;
  error?: string;
  host_id?: string;
  host_name?: string;
  node_id: string;
  node_ip?: string;
  country?: string;
  endpoint_address?: string;
  endpoint_port?: number;
  protocol?: string;
  transport?: string;
  flow?: string;
  updated_at?: string;
  profiles: CanonicalProfile[];
}

export interface AllClientConfigResponse {
  available: boolean;
  error?: string;
  nodes?: NodeClientConfig[];
  profiles?: CanonicalProfile[];
}

export interface ClientProfile {
  host_id: string;
  host_name: string;
  node_id: string;
  name: string;
  protocol: string;
  uri: string;
  xray_config?: Record<string, unknown>;
  singbox_config?: Record<string, unknown>;
  clash_config?: string;
  supported_clients: string[];
}

export interface DaemonStatus {
  server_id: string;
  name: string;
  region: string;
  version: string;
  status: string;
  uptime: number;
}

export interface SystemStats {
  OS: string;
  CPU: number;
  Memory: number;
  Uptime: number;
  RX: number;
  TX: number;
}

export interface CurrentExit {
  slot: number;
  status: string; // "ACTIVE"
  node_id: string;
  ip: string;
  country: string;
  region: string;
  score: number;
  throughput: number;
  last_check: string;
  reputation?: ReputationMetrics;
  network_class?: NetworkClass;
  host_id?: string;
  host_name?: string;
}

export interface SlotsOverview {
  total_configured: number;
  slots: Record<number, string>;
  manual_overrides: Record<number, boolean>;
}

export interface PoolSummary {
  active: number;
  standby: number;
  qualified: number;
  candidate: number;
  cooldown: number;
  rejected: number;
  pending_review?: number;
}

export interface DiscoveryRefreshStatus {
  running: boolean;
  source?: string;
  last_started?: string;
  last_completed?: string;
  last_error?: string;
  last_result: {
    fetched: number;
    accepted: number;
    rejected: number;
  };
}

export interface DiscoveryRefreshResponse {
  started: boolean;
  status: DiscoveryRefreshStatus;
}

export interface ReputationMetrics {
  status: string; // "GOOD", "BAD", "UNKNOWN", "RISKY"
  is_blacklisted: boolean;
  fraud_score: number;
  provider_name: string;
  details: string;
}

export interface NetworkClass {
  asn: string;
  isp: string;
  organization: string;
  network_type: string;
  is_vpn: boolean;
  is_proxy: boolean;
  is_tor: boolean;
  is_hosting: boolean;
}

export interface PerformanceMetrics {
  rtt_ms: number;
  throughput_bps: number;
  download_bps: number;
  upload_bps: number;
  upload_status: string;
  speed_status: string;
  packet_loss_pct: number;
  packet_loss_status: string;
  duration_ms: number;
  last_checked: string;
}

export interface Node {
  id: string;
  hostname: string;
  ip: string;
  score: number;
  country: string;
  country_long: string;
  sessions: number;
  uptime: number;
  users: number;
  message: string;
  total_traffic: number;
  log_type: string;
  operator: string;
  endpoint_host: string;
  endpoint_port: number;
  endpoint_proto: string;
  openvpn_config?: string;
  status: string;
  last_seen: string;
  first_seen: string;
  fail_count: number;
  last_error?: string;
  last_failure_at?: string;
  reputation: ReputationMetrics;
  network_class: NetworkClass;
  performance: PerformanceMetrics;
}

export interface Operation {
  operation_id: string;
  slot: number;
  target_node_id: string;
  status: 'REQUESTED' | 'PREPARING' | 'CONNECTING' | 'VERIFYING' | 'ACTIVE' | 'FAILED';
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface ShareLinkResult {
  protocol: string;
  supported: boolean;
  uri?: string;
  node_id: string;
  country: string;
  description: string;
  error?: string;
}

export interface Subscription {
  id: string;
  name: string;
  host_id?: string;
  profile: 'active_only' | 'all_nodes' | 'region' | 'protocol';
  region_filter?: string;
  protocol_filter?: string;
  is_revoked: boolean;
  created_at: string;
  expires_at?: string;
  created_by: string;
}

export interface CreateSubResponse {
  subscription: Subscription;
  raw_token: string;
  sub_url: string;
}

export interface AuditLog {
  id: number;
  timestamp: string;
  user: string;
  role: string;
  action: string;
  target: string;
  result: string;
  source_ip: string;
  details?: string;
}

export interface EventItem {
  timestamp: string;
  severity: 'INFO' | 'WARN' | 'ERROR';
  component: string;
  event: string;
  node?: string;
  slot?: string;
  reason: string;
}

export interface RoutingSlotInfo {
  slot: number;
  table_id: number;
  fwmark: number;
  interface: string;
  node_ip?: string;
  country?: string;
  status: string;
}

export interface RoutingOverview {
  slots: RoutingSlotInfo[];
  dns_leak_protected: boolean;
  ipv6_leak_protected: boolean;
}
