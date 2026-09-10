import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  addHostToManager,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-04: Canonical Client Configuration Center', () => {
  let manager: TestManagerServer;
  let daemon: MockDaemonInstance;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    // 1. Setup Mock Daemon with rich canonical profiles
    daemon = await createMockDaemon({
      id: 'daemon-tokyo',
      name: 'Tokyo-Fleet',
      region: 'JP',
      token: 'tokyo-secret-token-777',
      nodeId: 'node-jp-vless-alpha',
      nodeIp: '203.0.113.88',
      endpointAddress: 'jp-node.example.com',
      endpointPort: 443,
    });

    // 2. Start Manager & Bootstrap
    manager = await startTestManagerServer();
    const session = await bootstrapAndSetPassword(manager, adminPassword);

    // 3. Add Host
    await addHostToManager(manager, session, {
      name: 'Tokyo Host',
      address: 'jp-node.example.com',
      agent_url: daemon.url,
      token: daemon.token,
      is_default: true,
    });
  });

  test.afterEach(async () => {
    await manager.stop();
    await daemon.close();
  });

  test('validates canonical client config display, profiles, copy, download, QR modal, and zero manual parameter construction', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);

    // 1. Log in
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Navigate to Share Links (Client Config Center)
    await page.click('text=Share Links');
    await expect(page.locator('h1')).toContainText('MULTI-HOST CLIENT CONFIGURATION CENTER');

    // 3. Verify Node Metadata loaded directly from Daemon Canonical API
    await expect(page.locator('span:has-text("node-jp-vless-alpha")')).toBeVisible();
    await expect(page.locator('text=IP: 203.0.113.88')).toBeVisible();
    await expect(page.locator('text=Endpoint: jp-node.example.com:443')).toBeVisible();
    await expect(page.locator('text=VLESS REALITY / xtls-rprx-vision')).toBeVisible();

    // 4. Verify all 5 Canonical Profiles are rendered
    await expect(page.getByText('VLESS URI', { exact: true })).toBeVisible();
    await expect(page.getByText('Clash Meta', { exact: true })).toBeVisible();
    await expect(page.getByText('sing-box', { exact: true })).toBeVisible();
    await expect(page.getByText('Xray Core', { exact: true })).toBeVisible();
    await expect(page.getByText('Subscription URI', { exact: true })).toBeVisible();

    // 5. Test Copy All Profiles
    const copyAllBtn = page.locator('button:has-text("Copy All Profiles")');
    await expect(copyAllBtn).toBeEnabled();
    await copyAllBtn.click();
    await expect(page.locator('text=Copied All Profiles!')).toBeVisible();

    // 6. Test Individual Profile Copy
    const vlessCard = page.locator('div.bg-slate-900\\/90', { hasText: 'VLESS URI' });
    const copyVlessBtn = vlessCard.locator('button:has-text("Copy")');
    await copyVlessBtn.click();
    await expect(vlessCard.getByText('Copied', { exact: true })).toBeVisible();

    // 7. Test Download of Canonical Profile
    const clashCard = page.locator('div.bg-slate-900\\/90', { hasText: 'Clash Meta' });
    const downloadClashBtn = clashCard.locator('button:has-text("Download")');

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      downloadClashBtn.click(),
    ]);

    expect(download.suggestedFilename()).toBe('clash-meta.yaml');
    const downloadPath = await download.path();
    expect(downloadPath).toBeTruthy();

    // 8. Test QR Code Modal
    const qrBtn = vlessCard.locator('button:has-text("QR")');
    await expect(qrBtn).toBeVisible();
    await qrBtn.click();

    // Verify QR modal opens with canvas rendering and CLIENT SHARELINK
    await expect(page.locator('text=CLIENT SHARELINK')).toBeVisible();
    const qrCanvas = page.locator('canvas');
    await expect(qrCanvas).toBeVisible();

    // Close QR modal
    await page.locator('div.fixed.inset-0 button').first().click();
    await expect(page.locator('text=CLIENT SHARELINK')).not.toBeVisible();

    // 9. SECURITY & USABILITY AUDIT: Verify zero manual Reality construction inputs
    // The Manager must NEVER require users to type or construct SNI, fingerprint, public key, short id, or flow!
    await expect(page.locator('input[placeholder*="SNI"]')).toHaveCount(0);
    await expect(page.locator('input[placeholder*="fingerprint"]')).toHaveCount(0);
    await expect(page.locator('input[placeholder*="public_key"]')).toHaveCount(0);
    await expect(page.locator('input[placeholder*="short_id"]')).toHaveCount(0);
    await expect(page.locator('input[placeholder*="flow"]')).toHaveCount(0);
  });
});
