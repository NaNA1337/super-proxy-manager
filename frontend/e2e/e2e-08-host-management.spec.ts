import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  createMockDaemon,
  TestManagerServer,
  MockDaemonInstance,
} from './test-harness';

test.describe('E2E-08: Host Management Lifecycle & Live Connection Testing', () => {
  let manager: TestManagerServer;
  let daemonSydney: MockDaemonInstance;
  let daemonLondon: MockDaemonInstance;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    daemonSydney = await createMockDaemon({
      id: 'sydney-01',
      name: 'Sydney',
      region: 'AU',
      token: 'syd-token-secret-9999',
      nodeId: 'node-syd-01',
      nodeIp: '139.130.4.5',
      endpointAddress: 'syd.example.com',
      endpointPort: 443,
    });

    daemonLondon = await createMockDaemon({
      id: 'london-01',
      name: 'London',
      region: 'UK',
      token: 'ldn-token-secret-8888',
      nodeId: 'node-ldn-01',
      nodeIp: '51.140.0.1',
      endpointAddress: 'ldn.example.com',
      endpointPort: 443,
    });

    manager = await startTestManagerServer();
    await bootstrapAndSetPassword(manager, adminPassword);
  });

  test.afterEach(async () => {
    await manager.stop();
    await daemonSydney.close();
    await daemonLondon.close();
  });

  test('validates host creation wizard, connection test failure & success, credential isolation, and deletion', async ({ page }) => {
    // 1. Log in
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Navigate to Hosts Fleet
    await page.click('text=Hosts Fleet');
    await expect(page.locator('h1')).toContainText('SUPER-PROXY HOSTS');

    // 3. Open Add Host Modal
    await page.click('button:has-text("Add Host")');
    await expect(page.locator('h2:has-text("ADD SUPER-PROXY HOST")')).toBeVisible();

    // 4. Wizard Step 1: Basic Info
    await page.fill('input[placeholder*="Tokyo-01"]', 'Sydney-DC');
    await page.fill('input[placeholder*="jp01.example.com"]', 'syd.example.com');
    await page.click('button:has-text("Next: API Credentials")');

    // 5. Wizard Step 2: Failed Connection Test Scenario
    const fakeSecretToken = 'super-secret-unleaked-token-xyz123';
    await page.fill('input[type="url"]', 'https://127.0.0.1:49123'); // unreachable port
    await page.fill('input[type="password"]', fakeSecretToken);
    await page.click('button:has-text("Next: Test Connection")');

    // Step 3 shows failure
    await expect(page.locator('text=Connection Failed')).toBeVisible();

    // SECRET AUDIT: The secret token must NOT be leaked into the error message or rendered DOM
    const modalText = await page.locator('div.glass-panel').last().innerText();
    expect(modalText).not.toContain(fakeSecretToken);

    // 6. Go back and enter valid credentials
    await page.click('button:has-text("Back")');

    // Step 2: Valid Sydney Daemon
    await page.fill('input[type="url"]', daemonSydney.url);
    await page.fill('input[type="password"]', daemonSydney.token);
    await page.click('button:has-text("Next: Test Connection")');

    // Step 3 shows verified connection
    await expect(page.locator('text=Connected Successfully')).toBeVisible();
    await expect(page.locator('text=Daemon Version:')).toBeVisible();

    // Save Host
    await page.click('button:has-text("Save & Register Host")');

    // 7. Navigate back to Hosts Fleet tab and verify Sydney host appears in table
    await page.click('text=Hosts Fleet');
    await expect(page.locator('td', { hasText: 'Sydney-DC' })).toBeVisible();
    await expect(page.locator('td', { hasText: 'syd.example.com' })).toBeVisible();

    // 8. Add London Host
    await page.click('button:has-text("Add Host")');
    await page.fill('input[placeholder*="Tokyo-01"]', 'London-DC');
    await page.fill('input[placeholder*="jp01.example.com"]', 'ldn.example.com');
    await page.click('button:has-text("Next: API Credentials")');

    await page.fill('input[type="url"]', daemonLondon.url);
    await page.fill('input[type="password"]', daemonLondon.token);
    await page.click('button:has-text("Next: Test Connection")');

    await expect(page.locator('text=Connected Successfully')).toBeVisible();
    await page.click('button:has-text("Save & Register Host")');

    // Both hosts visible in fleet
    await expect(page.locator('td', { hasText: 'Sydney-DC' })).toBeVisible();
    await expect(page.locator('td', { hasText: 'London-DC' })).toBeVisible();

    // 9. Delete London Host
    page.on('dialog', (dialog) => dialog.accept());
    const londonRow = page.locator('tr', { hasText: 'London-DC' });
    await londonRow.locator('button[title="Delete Host"]').click();

    // Verify London removed from table
    await expect(page.locator('td', { hasText: 'London-DC' })).toHaveCount(0);
    await expect(page.locator('td', { hasText: 'Sydney-DC' })).toBeVisible();

    // 10. Refresh page -> Sydney-DC remains persisted in SQLite
    await page.reload();
    await page.click('text=Hosts Fleet');
    await expect(page.locator('td', { hasText: 'Sydney-DC' })).toBeVisible();
    await expect(page.locator('td', { hasText: 'London-DC' })).toHaveCount(0);
  });
});
