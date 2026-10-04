async (page) => {
    const portal = 'http://156.238.252.115:32795';
    const tavern = 'http://156.238.252.115:32796';
    const site = 'Yisama\u7684\u81ea\u7528\u7ad9';
    const release = 'risuai-entry-20261003-2300';
    const output = `D:/Open source project/V Visa-free/new-api/output/playwright/${release}`;
    const assets = `D:/Open source project/.local-builds/${release}/release/dist`;
    const now = Math.floor(Date.now() / 1000);
    const primaryGroup = '\u4e0d\u964d\u667aplus';
    const results = [];
    let generationRequests = 0;
    for (const scenario of [
        { width: 1440, height: 900, locale: 'zh-CN', chinese: true },
        { width: 390, height: 844, locale: 'zh-CN', chinese: true },
        { width: 320, height: 740, locale: 'zh-CN', chinese: true },
        { width: 1440, height: 900, locale: 'en-US', chinese: false },
    ]) {
        const { width, height, locale, chinese } = scenario;
        const context = await page.context().browser().newContext({
            viewport: { width, height }, locale, serviceWorkers: 'block',
        });
        const entry = await context.newPage();
        entry.setDefaultTimeout(30000);
        const errors = [];
        const storage = new Map();
        let authenticated = false;
        entry.on('pageerror', error => errors.push(error.message));
        try {
            await context.route('**/*', async route => {
                const request = route.request();
                const url = new URL(request.url());
                if (![portal, tavern].includes(url.origin)) return route.abort();
                if (/\/pg\/|chat\/completions|images\/generations|images\/edits/.test(url.pathname)) {
                    generationRequests++;
                    return route.fulfill({ status: 403, json: { message: 'Generation blocked' } });
                }
                if (url.origin === tavern && request.method() === 'GET') {
                    if (url.pathname === '/') return route.fulfill({ path: assets + '/index.html' });
                    if (url.pathname.startsWith('/assets/')
                        || ['/logo_typo_trans.png', '/favicon.ico', '/sw.js'].includes(url.pathname)) {
                        return route.fulfill({ path: assets + url.pathname });
                    }
                    if (url.pathname === '/sw/init') return route.fulfill({ body: 'ok' });
                }
                if (url.origin === portal && url.pathname === '/api/status') {
                    return route.fulfill({ json: { success: true, data: { system_name: site } } });
                }
                if (url.origin !== tavern || !url.pathname.startsWith('/api/')) return route.abort();
                if (!authenticated) return route.fulfill({ status: 401, json: { message: 'Sign in first' } });
                let data;
                if (url.pathname === '/api/user/auth/new-api') {
                    data = { userId: '990028', sid: 'browser-only-layout', accessExpiresAt: now + 900, portalUrl: portal };
                } else if (url.pathname === '/api/new-api/account') {
                    const group = url.searchParams.get('group') === 'chat' ? 'chat' : primaryGroup;
                    data = {
                        systemName: site,
                        account: { id: '990028', username: 'root', display_name: 'Root User', quota: 316920000, group: primaryGroup },
                        groups: {
                            [primaryGroup]: { desc: chinese ? '\u6d4b\u8bd5\u4e2d' : 'Testing', ratio: 1 },
                            chat: { desc: 'Fixture Chat', ratio: 1 },
                        },
                        group, models: group === primaryGroup ? ['codex-auto-review'] : ['fixture-chat'],
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
            await entry.getByRole('button', { name: 'Accept', exact: true }).click();
            if (chinese) await entry.getByRole('button', { name: /^\u2022\s*\u4e2d\u6587$/ }).click();
            const input = entry.getByRole('textbox').first();
            await input.fill('yisama');
            await input.press('Enter');
            const connection = chinese ? '\u4f60\u7684 yisama \u8d26\u53f7\u5df2\u8fde\u63a5\u3002' : 'Your yisama account is connected.';
            await entry.getByText(connection, { exact: true }).waitFor();
            await entry.getByRole('button', {
                name: chinese ? /^\u5f15\u5bfc\u6211\u5b8c\u6210\u8bbe\u7f6e/ : /Guide me to setup/,
            }).click();
            const title = chinese ? `\u8fde\u63a5\u5230 ${site}` : `Connect to ${site}`;
            const panel = entry.getByRole('heading', { name: title, exact: true }).locator('..');
            await panel.getByRole('combobox').last().waitFor();
            const groupOptions = await panel.getByRole('combobox').first().getByRole('option').allTextContents();
            if (groupOptions.length !== 2 || groupOptions[0] !== primaryGroup || groupOptions[1] !== 'chat') {
                throw new Error('Onboarding group names replaced by descriptions: ' + JSON.stringify(groupOptions));
            }
            await panel.scrollIntoViewIfNeeded();
            const text = await entry.locator('body').innerText();
            const welcome = chinese ? `\u6b22\u8fce\u4f7f\u7528\u300c${site}\u300d\u7684 Airisu` : `Welcome to ${site}'s Airisu`;
            if (!text.replaceAll('\u2019', "'").includes(welcome)
                || text.split(connection).length - 1 !== 1
                || !text.includes(chinese ? '\u4ec5\u53ef\u5728\u9152\u9986\u804a\u5929\u4e2d\u4f7f\u7528' : 'Tavern chat only')
                || !text.includes('Root User') || !text.includes('$633.84') || !text.includes('$1.00')) {
                throw new Error('Welcome, nickname or balances changed: ' + text);
            }
            if (await entry.getByRole('textbox').count() !== 0) throw new Error('Unused setup input remains');
            const layout = await panel.evaluate(element => {
                const rect = node => {
                    const r = node.getBoundingClientRect();
                    return { x: r.x, y: r.y, width: r.width, height: r.height };
                };
                return {
                    pageWidth: document.documentElement.clientWidth,
                    scrollWidth: document.documentElement.scrollWidth,
                    panel: rect(element),
                    values: [...element.querySelectorAll('dd')].map(rect),
                    labels: [...element.querySelectorAll('dt')].map(rect),
                    selectors: [...element.querySelectorAll('select')].map(rect),
                    button: rect(element.querySelector('button')),
                    overflowing: [...element.querySelectorAll('dt, dd, h2, button')].filter(node =>
                        node.scrollWidth > node.clientWidth + 1).map(node => node.textContent),
                };
            });
            if (layout.values.length !== 3 || layout.scrollWidth > layout.pageWidth
                || layout.overflowing.length || errors.length
                || Math.abs(layout.values[1].y - layout.values[2].y) > 1
                || layout.selectors.some(select => Math.abs(select.height - 44) > 1)) {
                throw new Error(JSON.stringify({ layout, errors }));
            }
            if (width >= 768 && (Math.abs(layout.values[0].y - layout.values[1].y) > 1
                || Math.abs(layout.selectors[0].y - layout.selectors[1].y) > 1)) {
                throw new Error('Desktop columns misaligned');
            }
            if (width < 768 && (layout.values[0].y >= layout.values[1].y
                || Math.abs(layout.button.width - layout.panel.width + 16) > 2)) {
                throw new Error('Mobile account or full-width button misaligned');
            }
            await panel.screenshot({ path: `${output}/panel-${chinese ? 'cn' : 'en'}-${width}.png` });
            await entry.screenshot({ path: `${output}/page-${chinese ? 'cn' : 'en'}-${width}.png`, fullPage: true });
            await panel.getByRole('combobox').first().selectOption('chat');
            await panel.getByRole('combobox').last().getByRole('option', { name: 'fixture-chat', exact: true }).waitFor({ state: 'attached' });
            if (await panel.getByRole('combobox').last().inputValue() !== 'fixture-chat') {
                throw new Error('Switched group did not select its valid chat model');
            }
            const start = panel.getByRole('button', { name: chinese ? '\u5f00\u59cb\u4f7f\u7528 RisuAI' : 'Start RisuAI', exact: true });
            await start.click();
            await entry.getByText('chat / fixture-chat', { exact: true }).waitFor();
            await entry.getByRole('textbox').waitFor();
            let settingsCheck = 'not applicable to this scenario';
            if (!chinese) {
                await entry.getByRole('button', { name: /^Full English/ }).click();
                await entry.getByRole('button', { name: /^Balanced/ }).click();
                await entry.locator('.rs-sidebar').waitFor();
                const sidebar = entry.locator('.rs-sidebar');
                await sidebar.getByRole('button').first().click();
                await sidebar.locator('button.ico').first().click();
                await entry.getByRole('button', { name: 'Chat Bot', exact: true }).click();
                const settings = entry.locator('.rs-setting-cont');
                await settings.getByRole('combobox').first().getByRole('option', { name: primaryGroup, exact: true }).waitFor({ state: 'attached' });
                const names = await settings.getByRole('combobox').first().getByRole('option').allTextContents();
                if (names[0] !== primaryGroup || names[1] !== 'chat') {
                    throw new Error('Model settings group names replaced by descriptions: ' + JSON.stringify(names));
                }
                await settings.getByRole('combobox').first().selectOption(primaryGroup);
                await settings.getByRole('combobox').last().getByRole('option', { name: 'codex-auto-review', exact: true }).waitFor({ state: 'attached' });
                await entry.screenshot({ path: `${output}/model-settings-${width}.png` });
                await settings.getByRole('button', { name: 'Account & Files', exact: true }).click();
                await settings.getByText('Group: ' + primaryGroup, { exact: true }).waitFor();
                await entry.screenshot({ path: `${output}/account-settings-${width}.png` });
                settingsCheck = 'Model settings options and account current-group label use names, not descriptions';
            }
            results.push({ scenario, layout, errors, welcome, connection, groupOptions, settingsCheck, continueWorks: true, groupSwitchWorks: true });
        } catch (error) {
            await entry.screenshot({ path: `${output}/failed-${chinese ? 'cn' : 'en'}-${width}.png`, fullPage: true });
            throw new Error(JSON.stringify({
                message: error.message, scenario, errors, generationRequests,
                text: (await entry.locator('body').innerText()).slice(0, 5000),
            }));
        } finally {
            await context.close();
        }
    }
    if (generationRequests !== 0) throw new Error('Generation attempted');
    return {
        results, generationRequests, paidCalls: 0, release,
        data: 'Synthetic browser-only account and in-memory storage; no backend account mutations',
        assets: 'New local release HTML and assets; deployed checksum verification is separate',
        publicAssetDownload: 'Not tested in this layout fixture',
    };
}
