const { Given, When, Then, Before, After } = require('@cucumber/cucumber');
const { chromium } = require('playwright');
const mockServer = require('../mock-server.cjs');

let browser;
let page;
let context;
let serverInstance;

Before(async function () {
  browser = await chromium.launch({ headless: true });
  context = await browser.newContext();
  page = await context.newPage();
  await page.setViewportSize({ width: 1280, height: 720 });
});

After(async function () {
  if (page) await page.close();
  if (context) await context.close();
  if (browser) await browser.close();
});

Given('I navigate to vault {string}', async function (vaultId) {
  await page.goto(`http://localhost:9999/#/vault/${vaultId}`, { waitUntil: 'load', timeout: 10000 });
  await page.waitForTimeout(1000);
});

When('I view the vault view', async function () {
  await page.waitForSelector('.vault-view', { timeout: 5000 });
});

Then('the vault view should load', async function () {
  const vaultView = await page.locator('.vault-view').isVisible();
  if (!vaultView) throw new Error('Vault view not visible');
});

Then('I should see the vault name {string} in the header', async function (vaultName) {
  const header = page.locator('.vault-name');
  await header.waitFor({ state: 'visible', timeout: 5000 });
  const text = await header.textContent();
  if (!text.includes(vaultName)) throw new Error(`Expected vault name ${vaultName}, got ${text}`);
});

Then('I should see the file tree sidebar', async function () {
  const sidebar = await page.locator('.sidebar').isVisible();
  if (!sidebar) throw new Error('Sidebar not visible');
});

Then('the file tree should be displayed', async function () {
  const fileTree = await page.locator('.file-tree').isVisible();
  if (!fileTree) throw new Error('File tree not displayed');
});

Then('I should see a connection status indicator', async function () {
  const indicator = await page.locator('.status-indicator').isVisible();
  if (!indicator) throw new Error('Connection status indicator not visible');
});

Then('I should see the main content area', async function () {
  const mainContent = await page.locator('.main-content').isVisible();
  if (!mainContent) throw new Error('Main content area not visible');
});
