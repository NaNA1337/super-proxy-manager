import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  addHostToManager,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-05: Runtime Unavailable Fail-Closed & Cache Invalidation', () => {
  let manager: TestManagerServer;
  let daemon: MockDaemonInstance;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    daemon = await createMockDaemon({
      id: 'daemon-fail-closed',
      name: 'Dynamic-Host',
      region: 'JP',
      token: 'secret-token-dyn-999',
      nodeId: 'node-dyn-alpha',
      nodeIp: '203.0.113.50',
      endpointAddress: 'dyn.example.com',
      endpointPort: 443,
    });

    manager = await startTestManagerServer();
    const session = await bootstrapAndSetPassword(manager, adminPassword);

    await addHostToManager(manager, session, {
      name: 'Dynamic Host',
      address: 'dyn.example.com',
      agent_url: daemon.url,
      token: daemon.token,
      is_default: true,
    });
  });

  test.afterEach(async () => {
    await manager.stop();
    await daemon.close();
  });

  test('validates fail-closed state, stale cache purging, zero fallback endpoints, and runtime recovery', async ({ page }) => {
    // 1. Log in
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Open Client Config (Share Links)
    await page.click('text=Share Links');
    await expect(page.locator('h1')).toContainText('MULTI-HOST CLIENT CONFIGURATION CENTER');

    // 3. Verify normal config is displayed initially
    await expect(page.locator('span:has-text("node-dyn-alpha")')).toBeVisible();
    await expect(page.getByText('VLESS URI', { exact: true })).toBeVisible();

    // 4. Stop Xray runtime on remote daemon (simulates daemon runtime endpoint 503 / down)
    daemon.setXrayOnline(false);

    // 5. Reload page -> Manager must detect unavailable runtime and purge stale cache immediately!
    await page.reload();
    await page.click('text=Share Links');

    // 6. Verification: Fail-closed warning banner is prominently displayed
    await expect(page.locator('text=Xray Unavailable / Client Configuration Temporarily Unavailable')).toBeVisible();
    await expect(page.locator('text=Security policy enforced: Stale links and previous cached configurations are strictly suppressed.')).toBeVisible();

    // 7. Verification: Stale profiles MUST NOT be displayed
    await expect(page.getByText('VLESS URI', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Clash Meta', { exact: true })).toHaveCount(0);
    await expect(page.getByText('sing-box', { exact: true })).toHaveCount(0);

    // 8. Verification: Zero fallback endpoints (no 127.0.0.1:1080 SOCKS fallback)
    await expect(page.locator('text=127.0.0.1:1080')).toHaveCount(0);
    await expect(page.locator('text=localhost:1080')).toHaveCount(0);

    // 9. Restore Xray runtime on daemon
    daemon.setXrayOnline(true);

    // 10. Reload page -> configuration recovers automatically
    await page.reload();
    await page.click('text=Share Links');

    await expect(page.locator('text=Xray Unavailable / Client Configuration Temporarily Unavailable')).toHaveCount(0);
    await expect(page.locator('span:has-text("node-dyn-alpha")')).toBeVisible();
    await expect(page.getByText('VLESS URI', { exact: true })).toBeVisible();
    await expect(page.getByText('Clash Meta', { exact: true })).toBeVisible();
  });
});
