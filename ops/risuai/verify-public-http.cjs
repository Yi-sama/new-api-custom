const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { chromium } = require('C:/Users/liurunsen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

const portal = 'http://156.238.252.115:32795';
const tavern = 'http://156.238.252.115:32796';
const output = path.resolve(__dirname, '../../output/playwright/risuai-us-http-20261003');

async function main() {
    await fs.mkdir(output, { recursive: true });
    const browser = await chromium.launch({ headless: true, channel: 'chrome' });
    let generationRequests = 0;
    const results = [];
    try {
        const signedOut = await browser.newContext({ viewport: { width: 1440, height: 900 } });
        const loginPage = await signedOut.newPage();
        await loginPage.goto(tavern + '/?connect=1');
        await loginPage.getByRole('link', { name: 'Sign in', exact: true }).waitFor();
        assert.equal(await loginPage.getByRole('link', { name: 'Sign in', exact: true }).getAttribute('href'), portal + '/sign-in?redirect=%2Ftavern');
        assert.equal(await loginPage.evaluate(() => window.isSecureContext), false);
        await loginPage.screenshot({ path: path.join(output, 'public-sign-in.png') });
        results.push({ check: 'Real public HTTP login boundary', pass: true, secureContext: false });
        await signedOut.close();

        for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
            const context = await browser.newContext({ viewport, locale: 'en-US' });
            const errors = [];
            const storage = new Map();
            await context.route('**/*', async route => {
                const request = route.request();
                const url = new URL(request.url());
                if (url.origin !== tavern) return route.abort();
                if (!url.pathname.startsWith('/api/') && !url.pathname.startsWith('/proxy')) return route.continue();
                let data;
                if (url.pathname.includes('chat/completions') || url.pathname.includes('images')) {
                    generationRequests++;
                    return route.fulfill({ status: 403, json: { error: 'Generation blocked by browser verification' } });
                }
                if (url.pathname === '/api/user/auth/new-api') {
                    data = { userId: '990003', sid: 'browser-fixture-only', accessExpiresAt: Date.now() / 1000 + 900, portalUrl: portal };
                } else if (url.pathname === '/api/new-api/account') {
                    const group = url.searchParams.get('group') === 'premium' ? 'premium' : 'default';
                    data = {
                        systemName: 'HTTP Test Fixture',
                        account: { id: 990003, username: 'browser_fixture', display_name: 'Synthetic Browser Fixture', group: 'default', quota: 5000000 },
                        groups: { default: { desc: 'Fixture Chat', ratio: 1 }, premium: { desc: 'Fixture Premium', ratio: 2 } },
                        group,
                        models: group === 'premium' ? ['fixture-premium'] : ['fixture-chat'],
                        quotaPerUnit: 500000, quotaDisplayType: 'USD', usdExchangeRate: 7,
                    };
                } else if (url.pathname === '/api/test_auth') {
                    data = { status: 'success' };
                } else if (url.pathname === '/api/list') {
                    data = { success: true, content: [...storage.keys()] };
                } else if (url.pathname === '/api/read') {
                    const key = request.headers()['file-path'];
                    return route.fulfill({ contentType: 'application/octet-stream', body: storage.get(key) || Buffer.alloc(0) });
                } else if (url.pathname === '/api/write') {
                    storage.set(request.headers()['file-path'], request.postDataBuffer());
                    data = { success: true };
                } else {
                    return route.fulfill({ status: 404, json: { message: 'Unavailable in browser fixture' } });
                }
                return route.fulfill({ json: data });
            });
            const page = await context.newPage();
            page.on('pageerror', error => errors.push(error.message));
            await page.goto(tavern + '/?connect=1');
            await page.getByRole('textbox').first().waitFor({ timeout: 60000 });
            await page.getByRole('button', { name: 'Accept', exact: true }).click();
            await page.getByRole('textbox').first().fill('HTTP Test User');
            await page.getByRole('textbox').first().press('Enter');
            await page.getByRole('button', { name: /Guide me to setup/ }).click();
            await page.getByRole('heading', { name: 'Connect to HTTP Test Fixture', exact: true }).waitFor();
            assert.equal(await page.getByRole('combobox').count(), 2);
            await page.getByRole('combobox').first().selectOption('premium');
            await page.waitForFunction(() => document.querySelectorAll('select')[1]?.value === 'fixture-premium');
            let text = await page.locator('body').innerText();
            assert(text.includes('Synthetic Browser Fixture'));
            assert(!text.includes('API Key'));
            assert(!text.includes('Legal documents not configured'));
            assert.equal(await page.evaluate(() => window.isSecureContext), false);
            await page.screenshot({ path: path.join(output, `synthetic-model-selection-${viewport.width}.png`) });
            await page.getByRole('button', { name: 'Start RisuAI', exact: true }).click();
            await page.getByRole('button', { name: /Full English/ }).click();
            await page.getByRole('button', { name: /Balanced/ }).click();
            await page.getByRole('heading', { name: 'Connect to HTTP Test Fixture', exact: true }).waitFor({ state: 'hidden' });
            try {
                const sidebar = page.locator('.rs-sidebar');
                if (viewport.width < 700) await page.locator('button:has(svg.lucide-arrow-right)').first().click();
                await sidebar.waitFor();
                await sidebar.getByRole('button').first().click();
                await sidebar.locator('button.ico').first().click();
                await page.getByRole('button', { name: 'Chat Bot', exact: true }).click();
                await page.getByText('HTTP Test Fixture', { exact: true }).waitFor();
            } catch (error) {
                await page.screenshot({ path: path.join(output, `failed-app-${viewport.width}.png`) });
                console.error(JSON.stringify({ text: await page.locator('body').innerText(), errors }));
                throw error;
            }
            text = await page.locator('body').innerText();
            assert(text.includes('Synthetic Browser Fixture'));
            assert(!text.includes('API Key'));
            assert(!text.includes('Provider URL'));
            await page.screenshot({ path: path.join(output, `synthetic-app-${viewport.width}.png`) });
            const dimensions = await page.evaluate(() => ({
                width: document.documentElement.clientWidth,
                scrollWidth: document.documentElement.scrollWidth,
                title: document.title,
            }));
            assert(dimensions.scrollWidth <= dimensions.width);
            assert.deepEqual(errors, []);
            results.push({ check: 'Synthetic onboarding and model switching using deployed assets', viewport, dimensions, errors, text: text.slice(0, 1000) });
            await context.close();
        }
        assert.equal(generationRequests, 0);
        await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify({ results, generationRequests, realAccountLogin: 'not tested' }, null, 2), 'utf8');
        console.log(JSON.stringify({ results, generationRequests, realAccountLogin: 'not tested' }, null, 2));
    } finally {
        await browser.close();
    }
}

main().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
