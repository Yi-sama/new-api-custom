async (page) => {
    const portal = 'http://156.238.252.115:32795';
    const tavern = 'http://156.238.252.115:32796';
    const now = Math.floor(Date.now() / 1000);
    const bundle = {
        access_token: 'browser-only-fixture',
        token_type: 'Bearer',
        access_expires_at: now + 900,
        user: {
            id: 990003, username: 'browser_fixture', display_name: 'Browser Fixture',
            role: 1, status: 1, quota: 5000000, group: 'default',
        },
        session: {
            sid: 'browser-only', current: true, login_method: 'password',
            ip: 'fixture', user_agent: 'fixture', created_at: now,
            last_active_at: now, expires_at: now + 3600,
        },
    };
    const results = [];
    let generationRequests = 0;
    for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
        const context = await page.context().browser().newContext({ viewport, locale: 'en-US' });
        const errors = [];
        const storage = new Map();
        let mainSignedIn = false;
        let tavernSignedIn = false;
        let stage = 'main sign-in';
        const entry = await context.newPage();
        entry.setDefaultTimeout(45000);
        try {
            await context.route('**/*', async route => {
                const request = route.request();
                const url = new URL(request.url());
                if (![portal, tavern].includes(url.origin)) return route.abort();
                if (/chat\/completions|images\/generations|images\/edits/.test(url.pathname)) {
                    generationRequests++;
                    return route.fulfill({ status: 403, json: { message: 'Generation blocked' } });
                }
                if (url.pathname.startsWith('/proxy')) {
                    return route.fulfill({ status: 403, json: { message: 'Proxy blocked' } });
                }
                if (!url.pathname.startsWith('/api/')) return route.continue();
                if (url.origin === portal) {
                    if (['/api/status', '/api/setup'].includes(url.pathname)
                        && request.method() === 'GET') return route.continue();
                    if (url.pathname === '/api/user/login') {
                        mainSignedIn = tavernSignedIn = true;
                        return route.fulfill({ json: { success: true, data: bundle } });
                    }
                    if (url.pathname === '/api/user/auth/refresh' && mainSignedIn) {
                        return route.fulfill({ json: { success: true, data: bundle } });
                    }
                    return route.fulfill({ status: 401, json: { success: false } });
                }
                if (!tavernSignedIn) return route.fulfill({ status: 401, json: { message: 'Sign in first' } });
                let data;
                if (url.pathname === '/api/user/auth/new-api') {
                    data = { userId: '990003', sid: 'browser-only', accessExpiresAt: now + 900, portalUrl: portal };
                } else if (url.pathname === '/api/new-api/account') {
                    data = {
                        systemName: 'Entry Test Fixture', account: bundle.user,
                        groups: { default: { desc: 'Fixture Chat', ratio: 1 } },
                        group: 'default', models: ['fixture-chat'],
                        quotaPerUnit: 500000, quotaDisplayType: 'USD', usdExchangeRate: 1,
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
            entry.on('pageerror', error => errors.push(error.message));
            entry.on('framenavigated', frame => {
                if (frame === entry.mainFrame() && new URL(frame.url()).pathname.startsWith('/dashboard')) {
                    errors.push('Unexpected dashboard navigation');
                }
            });
            stage = 'signed-out Tavern links to main sign-in';
            await entry.goto(tavern + '/?connect=1');
            const signIn = entry.getByRole('link', { name: 'Sign in', exact: true });
            await signIn.waitFor();
            const href = await signIn.getAttribute('href');
            if (href !== portal + '/sign-in?redirect=%2Ftavern') {
                throw new Error('Tavern sign-in link lost its return path');
            }
            await signIn.click();
            await entry.getByRole('button', { name: /^(Sign in|登录)$/ }).waitFor();
            if (new URL(entry.url()).searchParams.get('redirect') !== '/tavern') {
                throw new Error('Main sign-in lost its Tavern return path');
            }
            results.push({ viewport, check: 'Signed-out Tavern -> main sign-in with return path', url: entry.url() });

            stage = 'main Tavern entry sign-in';
            await entry.goto(portal + '/tavern');
            await entry.getByRole('button', { name: /^(Sign in|登录)$/ }).waitFor();
            if (new URL(entry.url()).searchParams.get('redirect') !== '/tavern') {
                throw new Error('Main entry lost the Tavern return path');
            }
            await entry.getByRole('textbox').first().fill('browser_fixture');
            await entry.locator('input[type=password]').fill('browser-only-fixture');
            await entry.getByRole('button', { name: /^(Sign in|登录)$/ }).click();
            stage = 'native Tavern after main sign-in';
            await entry.waitForURL(tavern + '/?connect=1');
            await entry.getByRole('textbox').first().waitFor({ timeout: 60000 });
            results.push({ viewport, check: 'Main entry -> sign in -> native Tavern', url: entry.url() });
            stage = 'native onboarding and managed account';
            await entry.getByRole('button', { name: 'Accept', exact: true }).click();
            await entry.getByRole('textbox').first().fill('Entry Fixture');
            await entry.getByRole('textbox').first().press('Enter');
            await entry.getByRole('button', { name: /Guide me to setup/ }).click();
            await entry.getByRole('heading', { name: 'Connect to Entry Test Fixture', exact: true }).waitFor();
            const selected = await entry.getByRole('combobox').allTextContents();
            if (!selected[0]?.includes('Fixture Chat') || !selected[1]?.includes('fixture-chat')) {
                throw new Error('Managed chat group or model was not rendered');
            }
            const dimensions = await entry.evaluate(() => ({
                width: document.documentElement.clientWidth,
                scrollWidth: document.documentElement.scrollWidth,
            }));
            if (dimensions.scrollWidth > dimensions.width || errors.length) {
                throw new Error(JSON.stringify({ dimensions, errors }));
            }
            await entry.screenshot({
                path: `D:/Open source project/V Visa-free/new-api/output/playwright/risuai-entry-20261003-1850/native-entry-${viewport.width}.png`,
            });
            results.push({ viewport, check: 'Native Tavern account and chat models render after main sign-in', url: entry.url(), dimensions, errors });
        } catch (error) {
            return {
                failed: true, stage, viewport, message: error.message, url: entry.url(),
                text: (await entry.locator('body').innerText()).slice(0, 1800),
                errors, results, generationRequests,
            };
        } finally {
            await context.close();
        }
    }
    if (generationRequests !== 0) throw new Error('Generation was attempted during verification');
    return { results, generationRequests, realAccountLogin: 'not tested' };
}
