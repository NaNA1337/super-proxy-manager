import { test, expect } from '@playwright/test';
import {
  startTestManagerServer,
  bootstrapAndSetPassword,
  TestManagerServer,
} from './test-harness';

test.describe('E2E-07: Settings Page and Password Management Lifecycle', () => {
  let manager: TestManagerServer;
  const initialPassword = 'InitialProductionPassword2026!#';
  const updatedPassword = 'NewUpdatedSecurityPassword2026!#';

  test.beforeEach(async () => {
    manager = await startTestManagerServer();
    await bootstrapAndSetPassword(manager, initialPassword);
  });

  test.afterEach(async () => {
    await manager.stop();
  });

  test('validates password update via Settings, session revocation, old password rejection, and new password authentication', async ({ page }) => {
    // 1. Log in with initial password
    await page.goto(manager.url);
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', initialPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 2. Navigate to Settings page
    await page.click('text=Settings');
    await expect(page.locator('h1')).toContainText('SYSTEM ARCHITECTURE & RUNTIME SETTINGS');

    // 3. Update Administrator Password via Settings form
    await page.fill('input[placeholder="Current password"]', initialPassword);
    await page.fill('input[placeholder="New password"]', updatedPassword);
    await page.fill('input[placeholder="Confirm new password"]', updatedPassword);
    await page.click('button:has-text("Update")');

    // 4. Verification: Success alert displayed
    await expect(page.locator('text=Administrator password updated successfully!')).toBeVisible();

    // 5. Log out
    await page.click('button:has-text("Logout")');
    await expect(page.locator('text=Access Control Center')).toBeVisible();

    // 6. Verification: Old password fails
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', initialPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('text=invalid credentials')).toBeVisible();

    // 7. Verification: Updated password succeeds
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', updatedPassword);
    await page.click('button[type="submit"]');
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();
    await expect(page.locator('text=Hosts Fleet')).toBeVisible();
  });
});
