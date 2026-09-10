import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  addHostToManager,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-10: TLS Certificate Pinning & Browser Secret Isolation Audit', () => {
  let manager: TestManagerServer;
  let daemonHttps: MockDaemonInstance;
  let sessionCookie: string;
  let csrfToken: string;
  const adminPassword = 'Production@AdminPass2026!#';
  const secretAgentToken = 'secret-agent-token-tls-pinning-9999';

  test.beforeEach(async () => {
    // Start HTTPS mock daemon with real self-signed TLS certificate
    daemonHttps = await createMockDaemon({
      id: 'daemon-tls',
      name: 'TLS-Secured-Host',
      region: 'US',
      token: secretAgentToken,
      nodeId: 'node-tls-alpha',
      nodeIp: '198.51.100.99',
      endpointAddress: 'secure-node.example.com',
      endpointPort: 443,
      useHttps: true,
    });

    manager = await startTestManagerServer();
    const auth = await bootstrapAndSetPassword(manager, adminPassword);
    sessionCookie = auth.sessionCookie;
    csrfToken = auth.csrfToken;
  });

  test.afterEach(async () => {
    await manager.stop();
    await daemonHttps.close();
  });

  test('validates TLS certificate fingerprint pinning: correct fingerprint succeeds, wrong fingerprint fails', async () => {
    expect(daemonHttps.fingerprint).toBeTruthy();

    // 1. Register host with CORRECT TLS fingerprint
    const validHost = await addHostToManager(manager, { sessionCookie, csrfToken }, {
      name: 'Valid-TLS-Host',
      address: 'secure-node.example.com',
      agent_url: daemonHttps.url,
      token: daemonHttps.token,
      is_default: true,
      tls_fingerprint: daemonHttps.fingerprint,
    });

    // Fetch client config -> should connect successfully using pinned certificate
    const validRes = await fetch(`${manager.url}/api/hosts/${validHost.id}/client-config`, {
      headers: { Cookie: `super_proxy_session=${sessionCookie}` },
    });
    expect(validRes.ok).toBe(true);
    const validData = await validRes.json();
    expect(validData.available).toBe(true);
    expect(validData.nodes[0].node_id).toBe('node-tls-alpha');

    // 2. Register host with TAMPERED / WRONG TLS fingerprint
    const wrongFingerprint = 'SHA256:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99';
    const tamperedHost = await addHostToManager(manager, { sessionCookie, csrfToken }, {
      name: 'Tampered-TLS-Host',
      address: 'secure-node.example.com',
      agent_url: daemonHttps.url,
      token: daemonHttps.token,
      is_default: false,
      tls_fingerprint: wrongFingerprint,
    });

    // Fetch client config -> must fail because of fingerprint mismatch
    const tamperedRes = await fetch(`${manager.url}/api/hosts/${tamperedHost.id}/client-config`, {
      headers: { Cookie: `super_proxy_session=${sessionCookie}` },
    });
    expect(tamperedRes.ok).toBe(true);
    const tamperedData = await tamperedRes.json();
    expect(tamperedData.available).toBe(false);
    expect(tamperedData.error.toLowerCase()).toContain('mismatch');
  });

  test('validates browser zero-secret leakage across network traffic, DOM, and browser storage', async ({ page }) => {
    // 1. Add valid host
    await addHostToManager(manager, { sessionCookie, csrfToken }, {
      name: 'Audit-TLS-Host',
      address: 'secure-node.example.com',
      agent_url: daemonHttps.url,
      token: secretAgentToken,
      is_default: true,
      tls_fingerprint: daemonHttps.fingerprint,
    });

    // 2. Record all HTTP responses sent to the browser
    const interceptedResponses: { url: string; body: string }[] = [];
    page.on('response', async (res) => {
      const url = res.url();
      if (url.includes('/api/')) {
        try {
          const body = await res.text();
          interceptedResponses.push({ url, body });
        } catch {}
      }
    });

    // 3. Navigate through the application
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // Visit key pages
    await page.click('text=Hosts Fleet');
    await expect(page.locator('td', { hasText: 'Audit-TLS-Host' })).toBeVisible();

    await page.click('text=Share Links');
    await expect(page.locator('span:has-text("node-tls-alpha")')).toBeVisible();

    await page.click('text=Settings');
    await expect(page.locator('h1')).toContainText('SYSTEM ARCHITECTURE & RUNTIME SETTINGS');

    await page.click('text=Audit Log');
    await expect(page.locator('h1')).toContainText('MUTATION AUDIT TRAIL');

    // 4. Verification: Check network responses for secret leakage
    expect(interceptedResponses.length).toBeGreaterThan(0);
    for (const item of interceptedResponses) {
      // Secret agent token must never be sent to browser
      expect(item.body).not.toContain(secretAgentToken);
      // Master key and raw credentials must never be sent to browser
      expect(item.body).not.toContain('MANAGER_MASTER_KEY');
      expect(item.body).not.toContain('encrypted_token');
      expect(item.body).not.toContain('enc:');
    }

    // 5. Verification: Check Browser localStorage and sessionStorage
    const storageDump = await page.evaluate(() => {
      return {
        local: JSON.stringify(window.localStorage),
        session: JSON.stringify(window.sessionStorage),
      };
    });

    expect(storageDump.local).not.toContain(secretAgentToken);
    expect(storageDump.local).not.toContain(adminPassword);
    expect(storageDump.local).not.toContain('MANAGER_MASTER_KEY');

    expect(storageDump.session).not.toContain(secretAgentToken);
    expect(storageDump.session).not.toContain(adminPassword);

    // 6. Verification: Check DOM & HTML source
    const pageContent = await page.content();
    expect(pageContent).not.toContain(secretAgentToken);
    expect(pageContent).not.toContain('MANAGER_MASTER_KEY');
  });
});
