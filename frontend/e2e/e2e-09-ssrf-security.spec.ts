import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  TestManagerServer,
} from './test-harness';

test.describe('E2E-09: Comprehensive SSRF Browser and API Defense Guard', () => {
  let manager: TestManagerServer;
  let sessionCookie: string;
  let csrfToken: string;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    // Start manager in STRICT production mode (ALLOW_PRIVATE_HOSTS=false)
    manager = await startTestManagerServer({ allowPrivateHosts: false });
    const auth = await bootstrapAndSetPassword(manager, adminPassword);
    sessionCookie = auth.sessionCookie;
    csrfToken = auth.csrfToken;
  });

  test.afterEach(async () => {
    await manager.stop();
  });

  test('validates that UI connection test rejects cloud metadata and non-http schemes', async ({ page }) => {
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', adminPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    await page.click('text=Hosts Fleet');
    await page.click('button:has-text("Add Host")');

    // Step 1
    await page.fill('input[placeholder*="Tokyo-01"]', 'Malicious-Host');
    await page.fill('input[placeholder*="jp01.example.com"]', '169.254.169.254');
    await page.click('button:has-text("Next: API Credentials")');

    // Step 2: Attempt cloud metadata target
    await page.fill('input[type="url"]', 'http://169.254.169.254/latest/meta-data');
    await page.fill('input[type="password"]', 'dummy-token');
    await page.click('button:has-text("Next: Test Connection")');

    // Verification: Backend SSRF guard blocks reachability check
    await expect(page.locator('text=Connection Failed')).toBeVisible();
    await expect(page.locator('text=prohibited')).toBeVisible();
  });

  test('validates that backend SSRF guard strictly blocks complete prohibited IP/scheme/URL matrix', async () => {
    const maliciousTargets = [
      { target: 'http://127.0.0.1:8080', desc: 'Loopback IPv4' },
      { target: 'http://localhost:8080', desc: 'Localhost domain' },
      { target: 'http://[::1]:8080', desc: 'Loopback IPv6' },
      { target: 'http://10.0.0.1:8080', desc: 'RFC1918 10.0.0.0/8' },
      { target: 'http://172.16.0.1:8080', desc: 'RFC1918 172.16.0.0/12' },
      { target: 'http://192.168.1.1:8080', desc: 'RFC1918 192.168.0.0/16' },
      { target: 'http://169.254.169.254/latest/meta-data', desc: 'AWS/GCP Link-Local Metadata' },
      { target: 'http://169.254.169.253', desc: 'AWS DNS Link-Local' },
      { target: 'http://100.64.0.1:8080', desc: 'Carrier Grade NAT 100.64.0.0/10' },
      { target: 'http://[::ffff:127.0.0.1]:8080', desc: 'IPv4-mapped IPv6 loopback' },
      { target: 'http://metadata.google.internal/computeMetadata/v1', desc: 'Google internal metadata hostname' },
      { target: 'http://admin:secret@evil.com:8080', desc: 'Userinfo embedded URL' },
      { target: 'file:///etc/passwd', desc: 'File scheme' },
      { target: 'gopher://127.0.0.1:6379/_', desc: 'Gopher scheme' },
      { target: 'ftp://backup.internal/config', desc: 'FTP scheme' },
    ];

    for (const item of maliciousTargets) {
      // 1. Test via Pre-Save Test API (/api/hosts/test)
      const testRes = await fetch(`${manager.url}/api/hosts/test`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken,
          'Cookie': `super_proxy_session=${sessionCookie}`,
        },
        body: JSON.stringify({
          agent_url: item.target,
          token: 'test-token',
        }),
      });

      expect(testRes.ok).toBe(true);
      const testData = await testRes.json();
      expect(testData.success).toBe(false);
      expect(testData.error.toLowerCase()).toMatch(/prohibited|invalid|scheme|userinfo|loopback|metadata/);

      // 2. Test via Host Creation API (/api/hosts)
      const createRes = await fetch(`${manager.url}/api/hosts`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken,
          'Cookie': `super_proxy_session=${sessionCookie}`,
        },
        body: JSON.stringify({
          name: `SSRF-Test-${item.desc}`,
          address: 'test.example.com',
          agent_url: item.target,
          token: 'test-token',
        }),
      });

      // Must be rejected with 400 Bad Request
      expect(createRes.status).toBe(400);
      const createBody = await createRes.text();
      expect(createBody.toLowerCase()).toMatch(/prohibited|invalid|scheme|userinfo|loopback|metadata/);
    }
  });
});
