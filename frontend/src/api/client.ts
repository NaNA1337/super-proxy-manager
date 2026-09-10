import {
  UserSession,
  DaemonStatus,
  SystemStats,
  CurrentExit,
  SlotsOverview,
  PoolSummary,
  Node,
  Operation,
  ShareLinkResult,
  Subscription,
  CreateSubResponse,
  AuditLog,
  EventItem,
  RoutingOverview
} from '../types';

let currentCSRFToken = '';

export function setCSRFToken(token: string) {
  currentCSRFToken = token;
}

export function getCSRFToken(): string {
  return currentCSRFToken;
}

export class APIError extends Error {
  status: number;
  safeMessage: string;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.safeMessage = message;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {});
  headers.set('Accept', 'application/json');

  if (options.method && ['POST', 'PUT', 'DELETE'].includes(options.method.toUpperCase())) {
    if (currentCSRFToken) {
      headers.set('X-CSRF-Token', currentCSRFToken);
    }
    if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
      headers.set('Content-Type', 'application/json');
    }
  }

  const response = await fetch(path, {
    ...options,
    headers,
    credentials: 'same-origin',
  });

  if (!response.ok) {
    let errorMsg = `HTTP Error ${response.status}`;
    try {
      const errJson = await response.json();
      if (errJson && errJson.error) {
        errorMsg = errJson.error;
      }
    } catch {
      // Non-JSON response
    }
    throw new APIError(response.status, errorMsg);
  }

  if (response.status === 204) {
    return {} as T;
  }

  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }

  return (await response.text()) as unknown as T;
}

export const api = {
  // Auth
  login: async (username: string, password: string) => {
    const res = await request<{ username: string; role: string; csrf_token: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    if (res.csrf_token) {
      setCSRFToken(res.csrf_token);
    }
    return res;
  },
  logout: async () => {
    return request<{ status: string }>('/api/auth/logout', { method: 'POST' });
  },
  getMe: async (): Promise<UserSession> => {
    const res = await request<UserSession>('/api/auth/me');
    if (res.csrf_token) {
      setCSRFToken(res.csrf_token);
    }
    return res;
  },

  // Daemon telemetry
  getStatus: () => request<DaemonStatus>('/api/daemon/status'),
  getSystem: () => request<SystemStats>('/api/daemon/system'),
  getCurrentExits: () => request<CurrentExit[]>('/api/daemon/current-exits'),
  getSlots: () => request<SlotsOverview>('/api/daemon/slots'),
  getPool: () => request<PoolSummary>('/api/daemon/pool'),
  getPoolQualified: () => request<Node[]>('/api/daemon/pool/qualified'),
  getNodes: (params?: { country?: string; status?: string; search?: string; limit?: number; offset?: number }) => {
    const q = new URLSearchParams();
    if (params?.country) q.set('country', params.country);
    if (params?.status) q.set('status', params.status);
    if (params?.search) q.set('search', params.search);
    if (params?.limit) q.set('limit', params.limit.toString());
    if (params?.offset) q.set('offset', params.offset.toString());
    const queryStr = q.toString() ? `?${q.toString()}` : '';
    return request<{ total: number; nodes: Node[] }>(`/api/daemon/nodes${queryStr}`);
  },
  getNodeDetails: (id: string) => request<Node>(`/api/daemon/nodes/${encodeURIComponent(id)}`),
  getMetricsText: () => request<string>('/api/daemon/metrics'),
  getRouting: () => request<RoutingOverview>('/api/daemon/routing'),

  // Mutations (Admin)
  switchSlot: (slot: number, nodeID: string) =>
    request<{ operation_id: string; status: string }>(`/api/daemon/slots/${slot}/switch`, {
      method: 'POST',
      body: JSON.stringify({ node_id: nodeID }),
    }),
  getOperation: (opID: string) => request<Operation>(`/api/daemon/operations/${encodeURIComponent(opID)}`),

  // ShareLinks
  getShareProtocols: () => request<{ supported: string[]; all: string[] }>('/api/sharelinks/protocols'),
  generateShareLink: (nodeID: string, protocol: string) =>
    request<ShareLinkResult>('/api/sharelinks/generate', {
      method: 'POST',
      body: JSON.stringify({ node_id: nodeID, protocol }),
    }),
  batchGenerateShareLinks: (protocols: string[]) =>
    request<ShareLinkResult[]>('/api/sharelinks/batch', {
      method: 'POST',
      body: JSON.stringify({ protocols }),
    }),

  // Subscriptions
  listSubscriptions: () => request<Subscription[]>('/api/subscriptions'),
  createSubscription: (data: { name: string; profile: string; region?: string; protocol?: string; duration_days?: number }) =>
    request<CreateSubResponse>('/api/subscriptions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  revokeSubscription: (id: string) =>
    request<{ status: string; id: string }>(`/api/subscriptions/${encodeURIComponent(id)}/revoke`, {
      method: 'POST',
    }),

  // Audit & Events & Settings
  getAuditLogs: () => request<AuditLog[]>('/api/audit'),
  getEvents: () => request<EventItem[]>('/api/events'),
  getSettings: () => request<Record<string, unknown>>('/api/settings'),
};
