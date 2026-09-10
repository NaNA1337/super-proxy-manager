import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  addHostToManager,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-03: Multi-Host Isolation & Dynamic Switching', () => {
  let manager: TestManagerServer;
  let daemonA: MockDaemonInstance;
  let daemonB: MockDaemonInstance;
  let hostA: { id: string; name: string };
  let hostB: { id: string; name: string };
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    // 1. Setup Mock Daemon A (Tokyo)
    daemonA = await createMockDaemon({
      id: 'tokyo-01',
      name: 'Tokyo',
      region: 'JP',
      token: 'tokyo-daemon-token-secret-111',
      nodeId: 'node-tokyo-alpha',
      nodeIp: '203.0.113.10',
      endpointAddress: 'tokyo.example.com',
      endpointPort: 443,
    });

    // 2. Setup Mock Daemon B (Seoul)
    daemonB = await createMockDaemon({
      id: 'seoul-01',
      name: 'Seoul',
      region: 'KR',
      token: 'seoul-daemon-token-secret-222',
      nodeId: 'node-seoul-beta',
      nodeIp: '198.51.100.22',
      endpointAddress: 'seoul.example.com',
      endpointPort: 443,
    });

    // 3. Start Manager & Bootstrap Admin
    manager = await startTestManagerServer();
    const session = await bootstrapAndSetPassword(manager, adminPassword);

    // 4. Register both hosts into Manager
    hostA = await addHostToManager(manager, session, {
      name: 'Tokyo',
      address: 'tokyo.example.com',
      agent_url: daemonA.url,
      token: daemonA.token,
      is_default: true,
    });

    hostB = await addHostToManager(manager, session, {
      name: 'Seoul',
      address: 'seoul.example.com',
      agent_url: daemonB.url,
      token: daemonB.token,
      is_default: false,
    });
  });

  test.afterEach(async () => {
    await manager.stop();
    await daemonA.close();
    await daemonB.close();
  });

  test('verifies multi-host fleet visibility, isolation, switching, and state non-pollution', async ({ page }) => {
    // 1. Login to Manager
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');

    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Navigate to Hosts Fleet
    await page.click('text=Hosts Fleet');
    await expect(page.locator('h1')).toContainText('SUPER-PROXY HOSTS');

    // Both hosts must be listed
    await expect(page.getByText('Tokyo', { exact: true })).toBeVisible();
    await expect(page.getByText('Seoul', { exact: true })).toBeVisible();

    // 3. Navigate to Client Config (Share Links) for Host A (Tokyo)
    await page.click('text=Share Links');
    await expect(page.locator('h1')).toContainText('MULTI-HOST CLIENT CONFIGURATION CENTER');

    // Wait for Tokyo node to be rendered
    await expect(page.locator('span:has-text("node-tokyo-alpha")')).toBeVisible();
    await expect(page.locator('span:has-text("tokyo.example.com")').first()).toBeVisible();

    // STRICT ISOLATION CHECK: Seoul node must NOT be rendered in Tokyo view
    await expect(page.locator('span:has-text("node-seoul-beta")')).toHaveCount(0);
    await expect(page.locator('span:has-text("seoul.example.com")')).toHaveCount(0);

    // 4. Switch Host to Host B (Seoul) using Navbar Host Dropdown
    await page.click('button:has-text("HOST:")');
    await page.getByRole('button', { name: /Seoul/ }).click();

    // 5. Verification: Content dynamically updates to Seoul
    await expect(page.locator('span:has-text("node-seoul-beta")')).toBeVisible();
    await expect(page.locator('span:has-text("seoul.example.com")').first()).toBeVisible();

    // STRICT ISOLATION CHECK: Tokyo node must NO LONGER be rendered
    await expect(page.locator('span:has-text("node-tokyo-alpha")')).toHaveCount(0);
    await expect(page.locator('span:has-text("tokyo.example.com")')).toHaveCount(0);

    // 6. Refresh page -> Host B selection persists across reloads
    await page.reload();
    await page.click('text=Share Links');
    await expect(page.locator('span:has-text("node-seoul-beta")')).toBeVisible();
    await expect(page.locator('span:has-text("seoul.example.com")').first()).toBeVisible();
    await expect(page.locator('span:has-text("node-tokyo-alpha")')).toHaveCount(0);
  });

  test('verifies backend API enforces strict host-level data isolation', async () => {
    // 1. Authenticate via API
    const loginRes = await fetch(`${manager.url}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'admin', password: adminPassword }),
    });
    const cookie = loginRes.headers.get('set-cookie')?.match(/super_proxy_session=([^;]+)/)?.[1] || '';

    // 2. Fetch Tokyo client config
    const tokyoRes = await fetch(`${manager.url}/api/hosts/${hostA.id}/client-config`, {
      headers: { Cookie: `super_proxy_session=${cookie}` },
    });
    expect(tokyoRes.ok).toBe(true);
    const tokyoData = await tokyoRes.json();
    expect(tokyoData.nodes).toHaveLength(1);
    expect(tokyoData.nodes[0].node_id).toBe('node-tokyo-alpha');
    // Ensure no Seoul data leaked
    const tokyoStr = JSON.stringify(tokyoData);
    expect(tokyoStr).not.toContain('seoul');
    expect(tokyoStr).not.toContain('node-seoul-beta');

    // 3. Fetch Seoul client config
    const seoulRes = await fetch(`${manager.url}/api/hosts/${hostB.id}/client-config`, {
      headers: { Cookie: `super_proxy_session=${cookie}` },
    });
    expect(seoulRes.ok).toBe(true);
    const seoulData = await seoulRes.json();
    expect(seoulData.nodes).toHaveLength(1);
    expect(seoulData.nodes[0].node_id).toBe('node-seoul-beta');
    // Ensure no Tokyo data leaked
    const seoulStr = JSON.stringify(seoulData);
    expect(seoulStr).not.toContain('tokyo');
    expect(seoulStr).not.toContain('node-tokyo-alpha');
  });
});
