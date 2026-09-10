import { test, expect } from '@playwright/test';
import { startTestManagerServer, TestManagerServer } from './test-harness';

test.describe('E2E-01: First Boot & Initialization Flow', () => {
  let manager: TestManagerServer;

  test.beforeEach(async () => {
    // Start fresh ephemeral manager instance with brand-new SQLite database
    manager = await startTestManagerServer();
  });

  test.afterEach(async () => {
    await manager.stop();
  });

  test('completes first boot, logs in with bootstrap password, forces password change, and persists session', async ({ page, browser }) => {
    // 1. Open login page
    await page.goto(manager.url);

    // Verify brand header and login form are present
    await expect(page.locator('h1')).toContainText('SUPER-PROXY');
    const usernameInput = page.locator('input[type="text"]');
    const passwordInput = page.locator('input[type="password"]');
    const submitButton = page.locator('button[type="submit"]');

    await expect(usernameInput).toBeVisible();
    await expect(passwordInput).toBeVisible();

    // 2. Try logging in with a wrong password
    await usernameInput.fill('admin');
    await passwordInput.fill('incorrect-password-123');
    await submitButton.click();

    // Error alert displayed
    await expect(page.locator('text=invalid credentials')).toBeVisible();

    // 3. Log in with the dynamically generated bootstrap password
    await usernameInput.fill('admin');
    await passwordInput.fill(manager.bootstrapPassword);
    await submitButton.click();

    // 4. Verification: MustChangePassword triggers ChangePassword screen
    await expect(page.locator('text=FIRST-TIME SETUP: PASSWORD CHANGE REQUIRED')).toBeVisible();
    await expect(page.locator('text=temporary bootstrap password')).toBeVisible();

    const currentPassInput = page.locator('input[placeholder*="temporary password"]');
    const newPassInput = page.locator('input[placeholder*="Minimum 12 characters"]');
    const confirmPassInput = page.locator('input[placeholder*="Re-enter new password"]');
    const changeSubmitButton = page.locator('button:has-text("Set Secure Password")');

    // 5. Test validation: password under 12 characters should be blocked by frontend
    await currentPassInput.fill(manager.bootstrapPassword);
    await newPassInput.fill('short');
    await confirmPassInput.fill('short');
    await changeSubmitButton.click();
    await expect(page.locator('text=New password must be at least 12 characters long')).toBeVisible();

    // 6. Test valid password update
    const newSecurePassword = 'NewProductionSecurePass2026!#';
    await newPassInput.fill(newSecurePassword);
    await confirmPassInput.fill(newSecurePassword);
    await changeSubmitButton.click();

    // 7. Verification: Password changed successfully, control panel unlocks!
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();
    await expect(page.locator('text=Hosts Fleet')).toBeVisible();

    // 8. Refresh page -> session persists (user remains logged in)
    await page.reload();
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();
    await expect(page.locator('text=Hosts Fleet')).toBeVisible();

    // 9. Log out
    const logoutButton = page.locator('button:has-text("Logout")');
    await logoutButton.click();
    await expect(page.locator('text=Access Control Center')).toBeVisible();

    // 10. Verification: Old bootstrap password no longer works!
    await usernameInput.fill('admin');
    await passwordInput.fill(manager.bootstrapPassword);
    await submitButton.click();
    await expect(page.locator('text=invalid credentials')).toBeVisible();

    // 11. Verification: New password logs in smoothly
    await usernameInput.fill('admin');
    await passwordInput.fill(newSecurePassword);
    await submitButton.click();
    await expect(page.locator('button:has-text("Logout")')).toBeVisible();

    // 12. Unauthenticated context cannot access protected pages
    const incognitoContext = await browser.newContext();
    const incognitoPage = await incognitoContext.newPage();
    await incognitoPage.goto(manager.url);
    await expect(incognitoPage.locator('text=Access Control Center')).toBeVisible();
    await expect(incognitoPage.locator('button:has-text("Logout")')).not.toBeVisible();
    await expect(incognitoPage.locator('text=Hosts Fleet')).not.toBeVisible();
    await incognitoContext.close();
  });
});
