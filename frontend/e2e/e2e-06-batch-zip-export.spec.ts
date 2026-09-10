import { test, expect } from '@playwright/test';
import AdmZip from 'adm-zip';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  addHostToManager,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-06: Batch ZIP Configuration Export', () => {
  let manager: TestManagerServer;
  let daemonA: MockDaemonInstance;
  let daemonB: MockDaemonInstance;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    daemonA = await createMockDaemon({
      id: 'daemon-tokyo',
      name: 'Tokyo',
      region: 'JP',
      token: 'tokyo-secret-token-111',
      nodeId: 'node-tokyo-alpha',
      nodeIp: '203.0.113.10',
      endpointAddress: 'tokyo.example.com',
      endpointPort: 443,
    });

    daemonB = await createMockDaemon({
      id: 'daemon-seoul',
      name: 'Seoul',
      region: 'KR',
      token: 'seoul-secret-token-222',
      nodeId: 'node-seoul-beta',
      nodeIp: '198.51.100.20',
      endpointAddress: 'seoul.example.com',
      endpointPort: 443,
    });

    manager = await startTestManagerServer();
    const session = await bootstrapAndSetPassword(manager, adminPassword);

    await addHostToManager(manager, session, {
      name: 'Tokyo',
      address: 'tokyo.example.com',
      agent_url: daemonA.url,
      token: daemonA.token,
      is_default: true,
    });

    await addHostToManager(manager, session, {
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

  test('validates multi-host batch ZIP export, directory layout, path safety, and zero secret leakage', async ({ page }) => {
    // 1. Log in
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Navigate to Share Links
    await page.click('text=Share Links');
    await expect(page.locator('h1')).toContainText('MULTI-HOST CLIENT CONFIGURATION CENTER');

    // 3. Click Batch Export ZIP
    await page.click('button:has-text("Batch Export ZIP")');

    // 4. Verification: Modal opens
    const modal = page.locator('div.fixed.inset-0');
    await expect(modal.locator('h2:has-text("BATCH EXPORT CLIENT CONFIGURATIONS")')).toBeVisible();

    // Tokyo is pre-selected by default. Select Seoul node as well.
    const seoulNodeCheck = modal.locator('span:has-text("node-seoul-beta")');
    await seoulNodeCheck.click();

    // Verify 2 nodes are selected
    await expect(modal.locator('text=Selected: 2 nodes')).toBeVisible();

    // 5. Trigger download of ZIP archive
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      modal.locator('button:has-text("Export Selected (ZIP)")').click(),
    ]);

    expect(download.suggestedFilename()).toMatch(/^super-proxy-configs-.*\.zip$/);
    const downloadPath = await download.path();
    expect(downloadPath).toBeTruthy();

    // 6. Security and Structure Analysis on ZIP contents
    const zip = new AdmZip(downloadPath!);
    const entries = zip.getEntries();
    expect(entries.length).toBeGreaterThanOrEqual(4);

    let foundTokyoVless = false;
    let foundSeoulSingbox = false;

    for (const entry of entries) {
      const entryName = entry.entryName;

      // PATH TRAVERSAL DEFENSE CHECKS
      expect(entryName).not.toContain('..');
      expect(entryName.startsWith('/')).toBe(false);
      expect(entryName.startsWith('\\')).toBe(false);

      if (entry.isDirectory) continue;

      const content = entry.getData().toString('utf8');

      // ZERO SECRET LEAKAGE CHECKS
      expect(content).not.toContain(daemonA.token);
      expect(content).not.toContain(daemonB.token);
      expect(content).not.toContain('MANAGER_MASTER_KEY');
      expect(content).not.toContain('private_key');
      expect(content).not.toContain('enc_key');

      // HOST ISOLATION IN ARCHIVE
      if (entryName.includes('Tokyo/node-tokyo-alpha/vless.txt')) {
        foundTokyoVless = true;
        expect(content).toContain('tokyo.example.com');
        expect(content).not.toContain('seoul.example.com');
      }

      if (entryName.includes('Seoul/node-seoul-beta/sing-box.json')) {
        foundSeoulSingbox = true;
        expect(content).toContain('seoul.example.com');
        expect(content).not.toContain('tokyo.example.com');
      }
    }

    expect(foundTokyoVless).toBe(true);
    expect(foundSeoulSingbox).toBe(true);
  });
});
