import { spawn, ChildProcess, execSync } from 'child_process';
import http from 'http';
import https from 'https';
import fs from 'fs';
import path from 'path';
import os from 'os';
import net from 'net';
import crypto from 'crypto';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export interface MockDaemonConfig {
  id: string;
  name: string;
  region: string;
  token: string;
  nodeId: string;
  nodeIp: string;
  endpointAddress: string;
  endpointPort: number;
  useHttps?: boolean;
}

export interface MockDaemonInstance {
  url: string;
  port: number;
  token: string;
  config: MockDaemonConfig;
  fingerprint?: string;
  setXrayOnline: (online: boolean) => void;
  close: () => Promise<void>;
}

export function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.listen(0, '127.0.0.1', () => {
      const port = (srv.address() as net.AddressInfo).port;
      srv.close(() => resolve(port));
    });
    srv.on('error', reject);
  });
}

export async function createMockDaemon(cfg: MockDaemonConfig): Promise<MockDaemonInstance> {
  const port = await getFreePort();
  let xrayOnline = true;
  let currentToken = cfg.token;

  const requestHandler = (req: http.IncomingMessage, res: http.ServerResponse) => {
    const urlObj = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`);
    const pathname = urlObj.pathname;

    // Control endpoints
    if (pathname === '/__control/set-xray-online') {
      let body = '';
      req.on('data', chunk => { body += chunk; });
      req.on('end', () => {
        try {
          const parsed = JSON.parse(body);
          xrayOnline = !!parsed.online;
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ ok: true, xrayOnline }));
        } catch {
          res.writeHead(400).end();
        }
      });
      return;
    }

    // Auth verification
    const authHeader = req.headers['authorization'];
    if (!authHeader || authHeader !== `Bearer ${currentToken}`) {
      res.writeHead(401, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'unauthorized: invalid agent token' }));
      return;
    }

    if (pathname === '/api/v1/status') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        status: 'healthy',
        version: '1.5.0',
        server_id: cfg.id,
        region: cfg.region,
        uptime: 3600,
      }));
      return;
    }

    if (pathname === '/api/v1/system') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        CPU: 12.5,
        Memory: 28.0,
        Load: [0.15, 0.25, 0.35],
      }));
      return;
    }

    if (pathname === '/api/v1/current-exits') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify([
        {
          node_id: cfg.nodeId,
          ip: cfg.nodeIp,
          country: cfg.region,
          status: 'ACTIVE',
        },
      ]));
      return;
    }

    if (pathname === '/api/v1/slots') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        active_slots: 1,
        total_slots: 5,
        slots: [
          {
            slot: 1,
            node_id: cfg.nodeId,
            ip: cfg.nodeIp,
            country: cfg.region,
            status: 'HEALTHY',
          },
        ],
      }));
      return;
    }

    if (pathname === '/api/v1/pool') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        total: 10,
        available: 8,
        active: 1,
      }));
      return;
    }

    if (pathname === '/api/v1/pool/qualified') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify([
        {
          id: cfg.nodeId,
          ip: cfg.nodeIp,
          country: cfg.region,
          status: 'QUALIFIED',
        },
      ]));
      return;
    }

    if (pathname === '/api/v1/nodes') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        total: 1,
        nodes: [
          {
            id: cfg.nodeId,
            ip: cfg.nodeIp,
            country: cfg.region,
            status: 'ACTIVE',
          },
        ],
      }));
      return;
    }

    if (pathname === '/api/v1/routing') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        rules: [
          { domain: 'geosite:cn', outbound: 'direct' },
          { ip: 'geoip:cn', outbound: 'direct' },
        ],
      }));
      return;
    }

    if (pathname === '/api/v1/metrics') {
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end('# HELP proxy_traffic_bytes_total Total traffic in bytes\nproxy_traffic_bytes_total 1024000\n');
      return;
    }

    if (pathname === '/api/v1/client-config/all') {
      if (!xrayOnline) {
        res.writeHead(503, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          error: 'runtime endpoint unavailable: Xray process stopped',
        }));
        return;
      }

      const vlessUri = `vless://uuid-${cfg.id}-test@${cfg.endpointAddress}:${cfg.endpointPort}?security=reality&sni=gateway.icloud.com&fp=chrome&pbk=testPbkBase64Val123#${cfg.name}-${cfg.nodeId}`;
      const clashYaml = `proxies:\n  - name: ${cfg.name}-${cfg.nodeId}\n    type: vless\n    server: ${cfg.endpointAddress}\n    port: ${cfg.endpointPort}\n    uuid: uuid-${cfg.id}-test\n    network: tcp\n    tls: true\n    reality-opts:\n      public-key: testPbkBase64Val123\n      server-name: gateway.icloud.com\n    client-fingerprint: chrome`;
      const singboxJson = JSON.stringify({
        type: 'vless',
        tag: `${cfg.name}-${cfg.nodeId}`,
        server: cfg.endpointAddress,
        server_port: cfg.endpointPort,
        uuid: `uuid-${cfg.id}-test`,
        tls: { enabled: true, server_name: 'gateway.icloud.com', reality: { enabled: true, public_key: 'testPbkBase64Val123' } },
      }, null, 2);
      const xrayJson = JSON.stringify({
        outbounds: [
          {
            protocol: 'vless',
            settings: {
              vnext: [
                {
                  address: cfg.endpointAddress,
                  port: cfg.endpointPort,
                  users: [{ id: `uuid-${cfg.id}-test`, encryption: 'none', flow: 'xtls-rprx-vision' }],
                },
              ],
            },
            streamSettings: { network: 'tcp', security: 'reality' },
          },
        ],
      }, null, 2);

      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        available: true,
        nodes: [
          {
            available: true,
            node_id: cfg.nodeId,
            node_ip: cfg.nodeIp,
            country: cfg.region,
            endpoint_address: cfg.endpointAddress,
            endpoint_port: cfg.endpointPort,
            protocol: 'vless',
            flow: 'xtls-rprx-vision',
            transport: 'tcp',
            updated_at: new Date().toISOString(),
            profiles: [
              {
                id: 'vless',
                name: 'VLESS URI',
                format: 'uri',
                filename: 'vless.txt',
                mime_type: 'text/plain',
                content: vlessUri,
                description: 'VLESS Reality / Vision Link',
                can_qr: true,
              },
              {
                id: 'clash',
                name: 'Clash Meta',
                format: 'yaml',
                filename: 'clash-meta.yaml',
                mime_type: 'application/x-yaml',
                content: clashYaml,
                description: 'Clash Meta Proxy Node Provider',
              },
              {
                id: 'sing-box',
                name: 'sing-box',
                format: 'json',
                filename: 'sing-box.json',
                mime_type: 'application/json',
                content: singboxJson,
                description: 'sing-box Outbound Specification',
              },
              {
                id: 'xray',
                name: 'Xray Core',
                format: 'json',
                filename: 'xray-config.json',
                mime_type: 'application/json',
                content: xrayJson,
                description: 'Xray Core Outbound Section',
              },
              {
                id: 'subscription',
                name: 'Subscription URI',
                format: 'uri',
                filename: 'subscription.txt',
                mime_type: 'text/plain',
                content: vlessUri,
                description: 'Raw Subscription Endpoint',
              },
            ],
          },
        ],
      }));
      return;
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not found' }));
  };

  let server: http.Server | https.Server;
  let fingerprint: string | undefined;

  if (cfg.useHttps) {
    const certDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cert-'));
    const keyPath = path.join(certDir, 'key.pem');
    const certPath = path.join(certDir, 'cert.pem');

    execSync(
      `openssl req -x509 -newkey rsa:2048 -keyout "${keyPath}" -out "${certPath}" -days 1 -nodes -subj "/CN=127.0.0.1"`,
      { stdio: 'ignore' }
    );

    const certDer = execSync(`openssl x509 -in "${certPath}" -outform DER`);
    const hash = crypto.createHash('sha256').update(certDer).digest('hex').toUpperCase();
    const parts: string[] = [];
    for (let i = 0; i < hash.length; i += 2) {
      parts.push(hash.slice(i, i + 2));
    }
    fingerprint = 'SHA256:' + parts.join(':');

    const key = fs.readFileSync(keyPath);
    const cert = fs.readFileSync(certPath);
    server = https.createServer({ key, cert }, requestHandler);

    // Clean up cert files on process exit
    fs.rmSync(certDir, { recursive: true, force: true });
  } else {
    server = http.createServer(requestHandler);
  }

  await new Promise<void>((resolve, reject) => {
    server.listen(port, '127.0.0.1', () => resolve());
    server.on('error', reject);
  });

  const scheme = cfg.useHttps ? 'https' : 'http';
  const url = `${scheme}://127.0.0.1:${port}`;

  return {
    url,
    port,
    token: cfg.token,
    config: cfg,
    fingerprint,
    setXrayOnline: (online: boolean) => {
      xrayOnline = online;
    },
    close: () => {
      return new Promise<void>((resolve) => {
        server.close(() => resolve());
      });
    },
  };
}

export interface TestManagerServer {
  url: string;
  port: number;
  dataDir: string;
  bootstrapPassword: string;
  stop: () => Promise<void>;
}

export async function startTestManagerServer(options: { allowPrivateHosts?: boolean } = {}): Promise<TestManagerServer> {
  const port = await getFreePort();
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'spm-e2e-data-'));
  const rootDir = path.resolve(__dirname, '../..');
  const binPath = path.join(rootDir, 'bin', 'super-proxy-web');

  // Ensure binary is compiled
  if (!fs.existsSync(binPath)) {
    execSync('go build -o bin/super-proxy-web ./backend/cmd/server', { cwd: rootDir, stdio: 'inherit' });
  }

  const env = {
    ...process.env,
    PORT: String(port),
    DATA_DIR: dataDir,
    ALLOW_PRIVATE_HOSTS: options.allowPrivateHosts !== false ? 'true' : 'false',
  };

  const proc = spawn(binPath, [], {
    env,
    cwd: rootDir,
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  let bootstrapPassword = '';
  let stdoutData = '';

  const passwordPromise = new Promise<string>((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error(`Timeout waiting for bootstrap password from manager process.\nStdout:\n${stdoutData}`));
    }, 15000);

    proc.stdout.on('data', (chunk) => {
      const text = chunk.toString();
      stdoutData += text;
      const match = text.match(/Temporary password:\s*([^\r\n]+)/);
      if (match) {
        bootstrapPassword = match[1].trim();
        clearTimeout(timeout);
        resolve(bootstrapPassword);
      }
    });

    proc.stderr.on('data', (chunk) => {
      stdoutData += chunk.toString();
    });

    proc.on('exit', (code) => {
      clearTimeout(timeout);
      reject(new Error(`Manager process exited prematurely with code ${code}.\nLogs:\n${stdoutData}`));
    });
  });

  await passwordPromise;

  // Poll until HTTP server is ready
  const managerUrl = `http://127.0.0.1:${port}`;
  const start = Date.now();
  let ready = false;

  while (Date.now() - start < 15000) {
    try {
      await new Promise<void>((resolve, reject) => {
        const req = http.get(`${managerUrl}/api/auth/bootstrap-status`, (res) => {
          if (res.statusCode === 200) {
            resolve();
          } else {
            reject(new Error(`status ${res.statusCode}`));
          }
        });
        req.on('error', reject);
        req.end();
      });
      ready = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 200));
    }
  }

  if (!ready) {
    proc.kill('SIGKILL');
    fs.rmSync(dataDir, { recursive: true, force: true });
    throw new Error(`Manager server failed to become responsive on ${managerUrl}`);
  }

  return {
    url: managerUrl,
    port,
    dataDir,
    bootstrapPassword,
    stop: async () => {
      proc.kill('SIGTERM');
      await new Promise((r) => setTimeout(r, 300));
      try {
        proc.kill('SIGKILL');
      } catch {}
      try {
        fs.rmSync(dataDir, { recursive: true, force: true });
      } catch {}
    },
  };
}

export async function bootstrapAndSetPassword(
  manager: TestManagerServer,
  newPassword = 'ProductionSecurePass2026!#'
): Promise<{ sessionCookie: string; csrfToken: string; password: string }> {
  // 1. Login with bootstrap password
  const loginRes = await fetch(`${manager.url}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'admin', password: manager.bootstrapPassword }),
  });
  if (!loginRes.ok) throw new Error(`Bootstrap login failed: ${loginRes.status}`);
  const loginData = await loginRes.json();
  const setCookie = loginRes.headers.get('set-cookie') || '';
  const cookieMatch = setCookie.match(/super_proxy_session=([^;]+)/);
  const cookie = cookieMatch ? cookieMatch[1] : '';

  // 2. Change password
  const changeRes = await fetch(`${manager.url}/api/auth/change-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': loginData.csrf_token,
      'Cookie': `super_proxy_session=${cookie}`,
    },
    body: JSON.stringify({
      current_password: manager.bootstrapPassword,
      new_password: newPassword,
      confirm_password: newPassword,
    }),
  });
  if (!changeRes.ok) throw new Error(`Bootstrap change-password failed: ${changeRes.status}`);
  const changeData = await changeRes.json();
  const newSetCookie = changeRes.headers.get('set-cookie') || setCookie;
  const newCookieMatch = newSetCookie.match(/super_proxy_session=([^;]+)/);

  return {
    sessionCookie: newCookieMatch ? newCookieMatch[1] : cookie,
    csrfToken: changeData.csrf_token || loginData.csrf_token,
    password: newPassword,
  };
}

export async function addHostToManager(
  manager: TestManagerServer,
  session: { sessionCookie: string; csrfToken: string },
  hostData: { name: string; address: string; agent_url: string; token: string; is_default?: boolean; tls_fingerprint?: string }
): Promise<{ id: string; name: string }> {
  const res = await fetch(`${manager.url}/api/hosts`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': session.csrfToken,
      'Cookie': `super_proxy_session=${session.sessionCookie}`,
    },
    body: JSON.stringify(hostData),
  });
  if (!res.ok) {
    const errText = await res.text();
    throw new Error(`Failed to add host: ${res.status} ${errText}`);
  }
  return await res.json();
}

