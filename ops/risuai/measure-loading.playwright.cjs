async (page) => {
    const tavern = 'http://156.238.252.115:32796';
    const output = 'D:/Open source project/V Visa-free/new-api/output/playwright/risuai-entry-20261003-2300';
    const context = await page.context().browser().newContext({
        viewport: { width: 1440, height: 900 }, locale: 'en-US', serviceWorkers: 'block',
    });
    const tab = await context.newPage();
    const requests = new Map();
    const results = [];
    const errors = [];
    const session = await context.newCDPSession(tab);
    await session.send('Network.enable');
    await session.send('Network.setBlockedURLs', {
        urls: ['https://*', '*://*/pg/*', '*://*/proxy*', '*chat/completions*', '*images/generations*'],
    });
    await context.addInitScript(({ tavern }) => {
        const nativeFetch = window.fetch.bind(window);
        const storage = new Map();
        window.__loadingFixture = { intercepted: 0, generation: 0 };
        window.fetch = async (input, options = {}) => {
            const url = new URL(typeof input === 'string' ? input : input instanceof Request ? input.url : input, location.href);
            if (/\/pg\/|chat\/completions|images\/generations|\/proxy/.test(url.pathname)) {
                window.__loadingFixture.generation++;
                throw new Error('Generation blocked by loading fixture');
            }
            if (url.pathname.startsWith('/api/')) {
                window.__loadingFixture.intercepted++;
                const json = data => new Response(JSON.stringify(data), { headers: { 'Content-Type': 'application/json' } });
                if (url.pathname === '/api/status') return json({ success: true, data: { system_name: 'Loading Fixture' } });
                if (url.pathname === '/api/user/auth/new-api') return json({
                    userId: '990041', sid: 'loading-browser-only', accessExpiresAt: Date.now() / 1000 + 900,
                    portalUrl: 'http://156.238.252.115:32795',
                });
                if (url.pathname === '/api/new-api/account') return json({
                    systemName: 'Loading Fixture', account: { id: '990041', username: 'fixture', display_name: 'Fixture', quota: 5000000, group: 'default' },
                    groups: { default: { desc: 'Description, not name', ratio: 1 } }, group: 'default',
                    models: ['fixture-chat'], quotaPerUnit: 500000, quotaDisplayType: 'USD', usdExchangeRate: 1,
                });
                if (url.pathname === '/api/test_auth') return json({ status: 'success' });
                if (url.pathname === '/api/list') return json({ success: true, content: [...storage.keys()] });
                const key = new Headers(options.headers || (input instanceof Request ? input.headers : {})).get('file-path');
                if (url.pathname === '/api/read') return new Response(storage.get(key) || new Uint8Array(), {
                    headers: { 'Content-Type': 'application/octet-stream' },
                });
                if (url.pathname === '/api/write') {
                    storage.set(key, options.body);
                    return json({ success: true });
                }
                return new Response('{}', { status: 404 });
            }
            if (url.origin !== tavern) throw new Error('External fetch blocked');
            return nativeFetch(input, options);
        };
    }, { tavern });
    tab.on('pageerror', error => errors.push(error.message));
    tab.on('request', request => requests.set(request.url(), { url: request.url(), done: false }));
    tab.on('requestfinished', request => {
        const row = requests.get(request.url());
        if (row) row.done = true;
    });
    try {
        for (const mode of ['cold', 'warm']) {
            requests.clear();
            const start = Date.now();
            await tab.goto(tavern + '/?connect=1', { waitUntil: 'commit', timeout: 20000 });
            let ready = true;
            try {
                await tab.getByRole('button', { name: 'Accept', exact: true }).waitFor({ timeout: 90000 });
            } catch {
                ready = false;
            }
            const elapsedMs = Date.now() - start;
            const timings = await tab.evaluate(() => ({
                fixture: window.__loadingFixture,
                resource: performance.getEntriesByType('resource').filter(r => r.name.includes('/assets/')).map(r => ({
                    url: r.name, duration: r.duration, transferSize: r.transferSize,
                    encodedBodySize: r.encodedBodySize, decodedBodySize: r.decodedBodySize,
                })),
            }));
            if (timings.fixture?.generation !== 0) throw new Error('Generation attempted');
            results.push({
                mode, ready, elapsedMs, ...timings, errors: [...errors],
                pending: [...requests.values()].filter(row => !row.done).map(row => row.url),
                networkApiRequests: [...requests.keys()].filter(url => new URL(url).pathname.startsWith('/api/')),
            });
            await tab.screenshot({ path: `${output}/public-loading-${mode}.png` });
            if (!ready) break;
        }
        return { results, data: 'Synthetic fetch responses; actual public static assets and browser HTTP cache, no Playwright routing', paidCalls: 0 };
    } finally {
        await context.close();
    }
}
