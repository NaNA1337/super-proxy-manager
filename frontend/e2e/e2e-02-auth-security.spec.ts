import { test, expect } from '@playwright/test';
import { startTestManagerServer, bootstrapAndSetPassword, TestManagerServer } from './test-harness';

test.describe('E2E-02: Authentication, Session, Logout, and CSRF Protection', () => {
  let manager: TestManagerServer;
  const adminPassword = 'Production@AdminPass2026!#';

  test.beforeEach(async () => {
    manager = await startTestManagerServer();
    await bootstrapAndSetPassword(manager, adminPassword);
  });

  test.afterEach(async () => {
    await manager.stop();
  });

  test('validates login, dashboard access, logout revocation, back/refresh protection, and wrong password handling', async ({ page }) => {
    // 1. Navigate to manager root
    await page.goto(manager.url);

    // Should display login screen
    await expect(page.locator('h1')).toContainText('SUPER-PROXY');
    const usernameInput = page.locator('input[type="text"]');
    const passwordInput = page.locator('input[type="password"]');
    const submitBtn = page.locator('button[type="submit"]');

    // 2. Attempt login with wrong password
    await usernameInput.fill('admin');
    await passwordInput.fill('IncorrectPass@123');
    await submitBtn.click();
    await expect(page.locator('text=invalid credentials')).toBeVisible();

    // 3. Login with valid password
    await usernameInput.fill('admin');
    await passwordInput.fill(adminPassword);
    await submitBtn.click();

    // Verification: Lands on Dashboard
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();
    await expect(page.locator('text=Hosts Fleet')).toBeVisible();

    // 4. Click Logout
    await page.locator('button:has-text("Logout")').click();

    // Verification: Redirected to login page
    await expect(page.locator('text=Access Control Center')).toBeVisible();
    await expect(page.locator('button:has-text("Logout")')).not.toBeVisible();

    // 5. Navigation protection: navigating to manager root still requires login
    await page.goto(manager.url);
    await expect(page.locator('text=Access Control Center')).toBeVisible();
    await expect(page.locator('button:has-text("Logout")')).not.toBeVisible();

    // 6. Refresh protection: reloading page remains unauthenticated
    await page.reload();
    await expect(page.locator('text=Access Control Center')).toBeVisible();
    await expect(page.locator('button:has-text("Logout")')).not.toBeVisible();
  });

  test('enforces CSRF defense against state-modifying requests without token', async () => {
    // 1. Log in via API to get session cookie
    const loginRes = await fetch(`${manager.url}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'admin', password: adminPassword }),
    });
    expect(loginRes.ok).toBe(true);
    const setCookie = loginRes.headers.get('set-cookie') || '';
    const cookieMatch = setCookie.match(/super_proxy_session=([^;]+)/);
    expect(cookieMatch).toBeTruthy();
    const sessionCookie = cookieMatch![1];

    // 2. Send POST request with valid session cookie BUT without X-CSRF-Token
    const noCsrfRes = await fetch(`${manager.url}/api/hosts`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Cookie': `super_proxy_session=${sessionCookie}`,
      },
      body: JSON.stringify({
        name: 'CSRF-Attack-Host',
        address: 'attack.example.com',
        agent_url: 'http://127.0.0.1:19999',
        token: 'token',
      }),
    });

    // Verification: CSRF middleware blocks request with 403 Forbidden
    expect(noCsrfRes.status).toBe(403);
    const body = await noCsrfRes.text();
    expect(body.toLowerCase()).toContain('csrf');
  });
});
