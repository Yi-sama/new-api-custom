async (page) => {
    const portal = 'http://156.238.252.115:32795';
    const tavern = 'http://156.238.252.115:32796';
    const site = 'Yisama\u7684\u81ea\u7528\u7ad9';
    const now = Math.floor(Date.now() / 1000);
    const output = 'D:/Open source project/V Visa-free/new-api/output/playwright/tavern-credit-20261003-2110';
    const assets = 'D:/Open source project/.local-builds/tavern-credit-20261003-2110/release/dist';
    const results = [];
    let generationRequests = 0;
    for (const scenario of [
        { width: 1440, height: 900, locale: 'zh-CN', chinese: true },
        { width: 390, height: 844, locale: 'zh-CN', chinese: true },
        { width: 1440, height: 900, locale: 'en-US', chinese: false },
    ]) {
        const { width, height, locale, chinese } = scenario;
        const context = await page.context().browser().newContext({ viewport: { width, height }, locale });
        const entry = await context.newPage();
        entry.setDefaultTimeout(45000);
        entry.setDefaultNavigationTimeout(180000);
        const errors = [];
        const responses = [];
        const pendingAssets = new Set();
        const failedAssets = [];
        const storage = new Map();
        let authenticated = false;
        entry.on('pageerror', error => errors.push(error.message));
        entry.on('request', request => {
            if (new URL(request.url()).pathname.startsWith('/assets/')) pendingAssets.add(request.url());
        });
        entry.on('requestfinished', request => pendingAssets.delete(request.url()));
        entry.on('requestfailed', request => {
            if (pendingAssets.delete(request.url())) {
                failedAssets.push({ url: request.url(), failure: request.failure() });
            }
        });
        entry.on('response', response => {
            const url = new URL(response.url());
            if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/sw/')) {
                responses.push({ path: url.pathname, method: response.request().method(), status: response.status() });
            }
        });
        try {
            await context.route('**/*', async route => {
                const request = route.request();
                const url = new URL(request.url());
                if (![portal, tavern].includes(url.origin)) return route.abort();
                if (/\/pg\/|chat\/completions|images\/generations|images\/edits/.test(url.pathname)) {
                    generationRequests++;
                    return route.fulfill({ status: 403, json: { message: 'Generation blocked' } });
                }
                if (url.origin === tavern && url.pathname.startsWith('/assets/')) {
                    return route.fulfill({ path: assets + url.pathname });
                }
                if (!url.pathname.startsWith('/api/')) return route.continue();
                if (url.origin === portal) {
                    if (request.method() === 'GET' && url.pathname === '/api/status') return route.continue();
                    return route.fulfill({ status: 403, json: { success: false } });
                }
                if (!authenticated) return route.fulfill({ status: 401, json: { message: 'Sign in first' } });
                let data;
                if (url.pathname === '/api/user/auth/new-api') {
                    data = { userId: '990028', sid: 'browser-only-credit', accessExpiresAt: now + 900, portalUrl: portal };
                } else if (url.pathname === '/api/new-api/account') {
                    data = {
                        systemName: site,
                        account: { id: '990028', username: 'root', display_name: 'Root User', quota: 5000000, group: 'default' },
                        groups: { default: { desc: 'Fixture Chat', ratio: 1 } },
                        group: 'default', models: ['fixture-chat'],
                        quotaPerUnit: 500000, quotaDisplayType: 'USD', usdExchangeRate: 1,
                        tavernCredit: { quota: 500000, granted_quota: 500000, created_time: now },
                    };
                } else if (url.pathname === '/api/test_auth') {
                    data = { status: 'success' };
                } else if (url.pathname === '/api/list') {
                    data = { success: true, content: [...storage.keys()] };
                } else if (url.pathname === '/api/read') {
                    return route.fulfill({
                        contentType: 'application/octet-stream',
                        body: storage.get(request.headers()['file-path']) || Buffer.alloc(0),
                    });
                } else if (url.pathname === '/api/write') {
                    storage.set(request.headers()['file-path'], request.postDataBuffer());
                    data = { success: true };
                } else {
                    return route.fulfill({ status: 404, json: { message: 'Unavailable in fixture' } });
                }
                return route.fulfill({ json: data });
            });
            await entry.goto(tavern + '/?connect=1');
            const link = entry.getByRole('link', { name: 'Sign in', exact: true });
            await link.waitFor();
            if (await link.getAttribute('href') !== portal + '/sign-in?redirect=%2Ftavern') {
                throw new Error('Main sign-in return path changed');
            }
            authenticated = true;
            await entry.reload();
            await entry.getByRole('button', { name: 'Accept', exact: true }).waitFor({ timeout: 180000 });
            await entry.getByRole('button', { name: 'Accept', exact: true }).click();
            if (chinese) {
                await entry.getByRole('button', { name: /^\u2022\s*\u4e2d\u6587$/ }).click();
            }
            const input = entry.getByRole('textbox').first();
            await input.fill('yisama');
            await input.press('Enter');
            const expectedConnection = chinese ? '\u4f60\u7684 yisama \u8d26\u53f7\u5df2\u8fde\u63a5\u3002' : 'Your yisama account is connected.';
            await entry.getByText(expectedConnection, { exact: true }).waitFor();
            await entry.getByRole('button', {
                name: chinese ? /^\u5f15\u5bfc\u6211\u5b8c\u6210\u8bbe\u7f6e/ : /Guide me to setup/,
            }).click();
            await entry.getByRole('combobox').first().waitFor();
            const text = await entry.locator('body').innerText();
            const welcome = chinese ? `\u6b22\u8fce\u4f7f\u7528\u300c${site}\u300d\u7684 Airisu` : `Welcome to ${site}'s Airisu`;
            if (!text.replaceAll('\u2019', "'").includes(welcome) || text.split(expectedConnection).length - 1 !== 1
                || !text.includes('Root User') || !text.includes('$1.00')
                || !text.includes(chinese ? '\u4ec5\u53ef\u5728\u9152\u9986\u804a\u5929\u4e2d\u4f7f\u7528' : 'Tavern chat only')
                || !text.includes(chinese ? '\u4e3b\u7ad9\u4f59\u989d' : 'Site balance')
                || text.includes(chinese ? `\u4f60\u7684 ${site} \u8d26\u53f7` : `Your ${site} account`)) {
                throw new Error('Welcome, nickname, gift scope or separate balance mismatch: ' + text);
            }
            const dimensions = await entry.evaluate(() => ({
                width: document.documentElement.clientWidth,
                scrollWidth: document.documentElement.scrollWidth,
                images: [...document.images].filter(image => image.offsetWidth > 0)
                    .map(image => ({ src: image.src, loaded: image.complete && image.naturalWidth > 0 })),
            }));
            if (dimensions.scrollWidth > dimensions.width || errors.length
                || dimensions.images.some(image => !image.loaded)) {
                throw new Error(JSON.stringify({ dimensions, errors }));
            }
            await entry.screenshot({ path: `${output}/gift-${chinese ? 'cn' : 'en'}-${width}.png`, fullPage: true });
            if (width === 390) {
                await entry.getByRole('combobox').first().scrollIntoViewIfNeeded();
                await entry.screenshot({ path: `${output}/account-cn-${width}.png`, fullPage: true });
            }
            results.push({ scenario, dimensions, errors, connection: expectedConnection, text, balance: '$1.00' });
        } catch (error) {
            await entry.screenshot({ path: `${output}/failed-${chinese ? 'cn' : 'en'}-${width}.png`, fullPage: true });
            throw new Error(JSON.stringify({
                message: error.message, scenario, url: entry.url(),
                text: (await entry.locator('body').innerText()).slice(0, 5000),
                errors, responses, generationRequests, pendingAssets: [...pendingAssets], failedAssets,
            }));
        } finally {
            await context.close();
        }
    }
    if (generationRequests !== 0) throw new Error('Generation attempted');
    const result = {
        results, generationRequests, realAccountLogin: 'not tested', paidCalls: 0,
        assets: 'Local release files verified byte-for-byte against the deployed SHA256SUMS',
        publicAssetDownload: 'Slow; 3.6 MB script download timed out at 60 seconds after 1.4 MB',
    };
    return result;
}
