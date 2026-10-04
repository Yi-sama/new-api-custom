const assert = require('node:assert/strict');
const { after, before, test } = require('node:test');
const { mkdtemp, rm } = require('node:fs/promises');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const express = require('express');
const { installNewApi } = require('./new-api.cjs');

const accounts = new Map([
    ['refresh-1', { id: '1', sid: 'sid-1', token: 'bearer-1', group: 'default', quota: 10 }],
    ['refresh-2', { id: '2', sid: 'sid-2', token: 'bearer-2', group: 'default', quota: 10 }],
]);
const debits = [];
const gifts = new Map();
const tavernKey = 'server-only-tavern-service-key-fixture-64-characters-not-client';
const publicHost = '156.238.252.115';
const publicOrigin = `http://${publicHost}:32796`;
const portalOrigin = `http://${publicHost}:32795`;
let dataDir;
let server;
let origin;
let lastUpstreamOrigin;

function response(status, body, headers = {}, cookies = []) {
    const encoded = body === undefined ? undefined : JSON.stringify(body);
    return {
        ok: status >= 200 && status < 300,
        status,
        headers: {
            get(name) { return headers[name.toLowerCase()] || null; },
            getSetCookie() { return cookies; },
        },
        body: encoded === undefined ? null : new ReadableStream({
            start(controller) { controller.enqueue(new TextEncoder().encode(encoded)); controller.close(); },
        }),
        json: async () => body,
    };
}

function accountFromAuthorization(headers) {
    const token = headers.Authorization?.replace('Bearer ', '');
    return [...accounts.values()].find(account => account.token === token);
}

async function upstream(url, init = {}) {
    const parsed = new URL(url);
    lastUpstreamOrigin = parsed.origin;
    const route = parsed.pathname.replace(/^\//, '');
    const account = accountFromAuthorization(init.headers || {});
    if (route === 'api/user/auth/refresh') {
        const refresh = /new_api_refresh=([^;]+)/.exec(init.headers?.Cookie || '')?.[1];
        const next = accounts.get(refresh);
        if (!next) return response(401, { success: false });
        if (init.headers?.['X-Auth-Session'] && init.headers['X-Auth-Session'] !== next.sid) {
            return response(401, { success: false });
        }
        return response(200, {
            success: true,
            data: {
                access_token: next.token,
                access_expires_at: Math.floor(Date.now() / 1000) + 3600,
                user: { id: next.id },
                session: { sid: next.sid },
            },
        }, {}, [`new_api_refresh=${refresh}; HttpOnly`]);
    }
    if (!account) return response(401, { success: false });
    if (route === 'api/user/self') {
        return response(200, { success: true, data: {
            id: account.id, username: `user-${account.id}`, display_name: `User ${account.id}`,
            quota: account.quota, group: account.group, status: account.revoked ? 0 : 1,
            revoked: Boolean(account.revoked), active: !account.revoked,
        } });
    }
    if (route === 'api/user/self/groups') {
        return response(200, { success: true, data: { default: {}, premium: {}, images: {}, 'auto-only': {} } });
    }
    if (route === 'api/user/models') {
        const group = parsed.searchParams.get('group');
        assert.equal(parsed.searchParams.get('modality'), 'chat');
        if (account.noChat) return response(200, { success: true, data: [] });
        if (group === 'images') return response(200, { success: true, data: [] });
        if (group === 'auto-only') return response(200, { success: true, data: ['Auto'] });
        return response(200, { success: true, data: group === 'premium' ? ['premium-model'] : ['default-model'] });
    }
    if (route === 'api/status') return response(200, { success: true, data: {
        system_name: 'Fixture Portal', quota_per_unit: 500000, quota_display_type: 'USD',
    } });
    if (route === 'pg/tavern/credit') {
        assert.equal(init.headers['X-Tavern-Service-Key'], tavernKey);
        const newlyGranted = !gifts.has(account.id) && init.method === 'POST';
        if (newlyGranted) gifts.set(account.id, { quota: 500000, granted_quota: 500000, created_time: 1 });
        return response(200, { success: true, data: { ...gifts.get(account.id), newly_granted: newlyGranted } });
    }
    if (route === 'api/user/auth/logout') {
        return response(200, { success: true }, {}, ['new_api_refresh=; Max-Age=0; HttpOnly']);
    }
    if (route === 'pg/chat/completions') {
        assert.equal(init.headers['X-Tavern-Service-Key'], tavernKey);
        const body = JSON.parse(init.body);
        debits.push({ userId: account.id, body });
        const gift = gifts.get(account.id);
        if (gift?.quota > 0) gift.quota -= 1;
        else account.quota -= 1;
        return {
            ok: true,
            status: 200,
            headers: { get(name) { return name === 'content-type' ? 'text/event-stream' : null; }, getSetCookie() { return []; } },
            body: new ReadableStream({
                start(controller) {
                    controller.enqueue(new TextEncoder().encode('data: {"choices":[{"delta":{"content":"ok"}}]}\n\n'));
                    controller.enqueue(new TextEncoder().encode('data: [DONE]\n\n'));
                    controller.close();
                },
            }),
        };
    }
    return response(404, { success: false });
}

function request(method, pathname, { headers = {}, body } = {}) {
    return new Promise((resolve, reject) => {
        const target = new URL(pathname, origin);
        const req = http.request(target, {
            method,
            headers: { Host: new URL(publicOrigin).host, ...(method === 'GET' ? {} : { Origin: publicOrigin }), ...headers },
        }, res => {
            const chunks = [];
            res.on('data', chunk => chunks.push(chunk));
            res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
        });
        req.on('error', reject);
        if (body) req.write(body);
        req.end();
    });
}

function json(result) { return JSON.parse(result.body.toString('utf8')); }

function sessionCookie(result) {
    const values = result.headers['set-cookie'] || [];
    const value = values.find(cookie => cookie.startsWith('risu_new_api='));
    assert.ok(value, 'bridge session cookie must be issued');
    return value.split(';', 1)[0];
}

function pinned(cookie, account) {
    return {
        Cookie: cookie,
        'X-New-API-User': account.id,
        'X-New-API-Session': account.sid,
    };
}

async function bootstrap(refresh) {
    const result = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: `new_api_refresh=${refresh}` },
    });
    assert.equal(result.status, 200);
    return { result, cookie: sessionCookie(result), account: accounts.get(refresh) };
}

before(async () => {
    dataDir = await mkdtemp(path.join(os.tmpdir(), 'risu-new-api-test-'));
    const app = express();
    app.use(express.json());
    app.use(express.raw({ type: 'application/octet-stream' }));
    installNewApi(app, {
        backendUrl: 'http://127.0.0.1:3000',
        portalUrl: portalOrigin,
        publicUrl: publicOrigin,
        allowPublicHttp: true,
        dataDir,
        secret: 'test-secret',
        tavernKey,
        request: upstream,
    });
    server = await new Promise(resolve => {
        const value = app.listen(0, '127.0.0.1', () => resolve(value));
    });
    origin = `http://127.0.0.1:${server.address().port}`;
});

after(async () => {
    await new Promise(resolve => server.close(resolve));
    await rm(dataDir, { recursive: true, force: true });
});

test('public HTTP bootstrap returns only session metadata and never the bearer', async () => {
    const { result } = await bootstrap('refresh-1');
    const body = json(result);
    assert.deepEqual(Object.keys(body).sort(), ['accessExpiresAt', 'portalUrl', 'sid', 'userId']);
    assert.equal(body.portalUrl, portalOrigin);
    assert.equal(lastUpstreamOrigin, 'http://127.0.0.1:3000');
    assert.ok(!result.body.includes('bearer-1'));
    assert.ok(!(result.headers['set-cookie'] || []).join(';').includes('bearer-1'));
});

test('missing or hostile-origin bootstrap authentication is denied', async () => {
    const missing = await request('POST', '/api/user/auth/new-api');
    const crossOrigin = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: 'new_api_refresh=refresh-1', Origin: 'http://attacker.invalid' },
    });
    const wrongHost = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: 'new_api_refresh=refresh-1', Host: 'attacker.invalid', Origin: publicOrigin },
    });
    assert.equal(missing.status, 401);
    assert.equal(crossOrigin.status, 403);
    assert.equal(wrongHost.status, 403);
});

test('public HTTP requires explicit opt-in and the backend remains loopback-only', () => {
    const options = {
        backendUrl: 'http://127.0.0.1:3000', portalUrl: portalOrigin, publicUrl: publicOrigin,
        dataDir, secret: 'config-test', request: upstream,
    };
    assert.throws(() => installNewApi(express(), { ...options, allowPublicHttp: false }), /loopback/);
    assert.throws(() => installNewApi(express(), {
        ...options, backendUrl: `http://${publicHost}:32795`, allowPublicHttp: true,
    }), /loopback/);
});

test('public HTTP env opt-in still rejects mixed schemes and malformed origins', () => {
    const previous = process.env.NEW_API_ALLOW_PUBLIC_HTTP;
    process.env.NEW_API_ALLOW_PUBLIC_HTTP = '1';
    const options = {
        backendUrl: 'http://127.0.0.1:3000', portalUrl: portalOrigin, publicUrl: publicOrigin,
        dataDir, secret: 'env-test', request: upstream,
    };
    try {
        assert.doesNotThrow(() => installNewApi(express(), options));
        assert.throws(() => installNewApi(express(), {
            ...options, portalUrl: 'http://127.0.0.1:32795',
        }), /same hostname/);
        for (const publicUrl of [
            `http://user:pass@${publicHost}:32796`,
            `${publicOrigin}/risu`,
            `https://${publicHost}:32796`,
            'not-an-origin',
        ]) {
            assert.throws(() => installNewApi(express(), { ...options, publicUrl }));
        }
    } finally {
        if (previous === undefined) delete process.env.NEW_API_ALLOW_PUBLIC_HTTP;
        else process.env.NEW_API_ALLOW_PUBLIC_HTTP = previous;
    }
});

test('public HTTPS bootstrap and logout use Secure cookies while backend stays HTTP loopback-only', async () => {
    const httpsOrigin = 'https://api.yisama.asia';
    const options = {
        backendUrl: 'http://127.0.0.1:3000', portalUrl: httpsOrigin, publicUrl: httpsOrigin,
        dataDir, secret: 'https-test', request: upstream,
    };
    assert.throws(() => installNewApi(express(), {
        ...options, backendUrl: 'https://api.yisama.asia',
    }));
    const app = express();
    app.use(express.json());
    installNewApi(app, options);
    const httpsServer = await new Promise(resolve => {
        const value = app.listen(0, '127.0.0.1', () => resolve(value));
    });
    const target = `http://127.0.0.1:${httpsServer.address().port}`;
    try {
        const headers = { Host: 'api.yisama.asia', Origin: httpsOrigin, Cookie: 'new_api_refresh=refresh-1' };
        const result = await request('POST', target + '/api/user/auth/new-api', { headers });
        assert.equal(result.status, 200);
        const cookies = result.headers['set-cookie'];
        const issued = cookies.find(value => value.startsWith('risu_new_api='));
        assert.ok(issued?.includes('; Secure'));
        assert.ok(issued?.includes('; HttpOnly'));
        assert.ok(issued?.includes('; SameSite=Strict'));
        const forbidden = await request('POST', target + '/api/user/auth/new-api', {
            headers: { ...headers, Origin: 'https://attacker.invalid' },
        });
        assert.equal(forbidden.status, 403);
        const account = accounts.get('refresh-1');
        const logout = await request('POST', target + '/api/user/auth/new-api/logout', {
            headers: {
                ...headers, ...pinned(issued.split(';', 1)[0], account),
                Cookie: issued.split(';', 1)[0] + '; new_api_refresh=refresh-1',
            },
        });
        assert.equal(logout.status, 200);
        assert.ok(logout.headers['set-cookie'].some(value =>
            value.startsWith('risu_new_api=;') && value.includes('; Secure')));
    } finally {
        await new Promise(resolve => httpsServer.close(resolve));
    }
});

test('normal refresh honors its SID and a stale refresh can reconnect without revoking the account', async () => {
    const session = await bootstrap('refresh-1');
    const normal = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: `${session.cookie}; new_api_refresh=refresh-1` },
    });
    assert.equal(normal.status, 200);
    session.account.sid = 'sid-1-next';
    const stale = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: `${session.cookie}; new_api_refresh=refresh-1` },
    });
    const reconnect = await request('POST', '/api/user/auth/new-api', {
        headers: { Cookie: `${session.cookie}; new_api_refresh=refresh-1`, 'Content-Type': 'application/json' },
        body: Buffer.from(JSON.stringify({ reconnect: true })),
    });
    assert.equal(stale.status, 401);
    assert.equal(reconnect.status, 200);
    const current = { cookie: sessionCookie(reconnect), account: session.account };
    const authenticated = await request('GET', '/api/test_auth', { headers: pinned(current.cookie, current.account) });
    assert.equal(authenticated.status, 200);
    session.account.sid = 'sid-1';
});

test('binary storage is isolated per authenticated user', async () => {
    const first = await bootstrap('refresh-1');
    const second = await bootstrap('refresh-2');
    const key = Buffer.from('private.bin').toString('hex');
    const write = await request('POST', '/api/write/', {
        headers: { ...pinned(first.cookie, first.account), 'file-path': key, 'Content-Type': 'application/octet-stream' },
        body: Buffer.from('user-one'),
    });
    assert.equal(write.status, 200);
    const own = await request('GET', '/api/read/', { headers: { ...pinned(first.cookie, first.account), 'file-path': key } });
    const other = await request('GET', '/api/read/', { headers: { ...pinned(second.cookie, second.account), 'file-path': key } });
    assert.equal(own.body.toString(), 'user-one');
    assert.equal(other.body.length, 0);
});

test('a 120-byte ASCII storage key roundtrips', async () => {
    const session = await bootstrap('refresh-1');
    const key = 'k'.repeat(120);
    const encoded = Buffer.from(key).toString('hex');
    const write = await request('POST', '/api/write/', {
        headers: { ...pinned(session.cookie, session.account), 'file-path': encoded, 'Content-Type': 'application/octet-stream' },
        body: Buffer.from('long-key'),
    });
    assert.equal(write.status, 200);
    const read = await request('GET', '/api/read/', { headers: { ...pinned(session.cookie, session.account), 'file-path': encoded } });
    assert.equal(read.body.toString(), 'long-key');
});

test('multi-key removal accepts a header longer than one key without crossing users', async () => {
    const first = await bootstrap('refresh-1');
    const second = await bootstrap('refresh-2');
    const keys = ['a'.repeat(70), 'b'.repeat(70)];
    for (const key of keys) {
        const encoded = Buffer.from(key).toString('hex');
        for (const [session, value] of [[first, 'first'], [second, 'second']]) {
            const write = await request('POST', '/api/write/', {
                headers: { ...pinned(session.cookie, session.account), 'file-path': encoded, 'Content-Type': 'application/octet-stream' },
                body: Buffer.from(value),
            });
            assert.equal(write.status, 200);
        }
    }
    const removalKey = Buffer.from(keys.join('$$')).toString('hex');
    assert.ok(removalKey.length > 240);
    const removed = await request('POST', '/api/remove/', {
        headers: { ...pinned(first.cookie, first.account), 'file-path': removalKey },
    });
    assert.equal(removed.status, 200);
    for (const key of keys) {
        const encoded = Buffer.from(key).toString('hex');
        const own = await request('GET', '/api/read/', { headers: { ...pinned(first.cookie, first.account), 'file-path': encoded } });
        const other = await request('GET', '/api/read/', { headers: { ...pinned(second.cookie, second.account), 'file-path': encoded } });
        assert.equal(own.body.length, 0);
        assert.equal(other.body.toString(), 'second');
    }
});

test('a stale tab pin is rejected before account access', async () => {
    const session = await bootstrap('refresh-1');
    const result = await request('GET', '/api/test_auth', {
        headers: { ...pinned(session.cookie, session.account), 'X-New-API-Session': 'stale' },
    });
    assert.equal(result.status, 409);
});

test('a revoked New API account loses bridge access', async () => {
    const session = await bootstrap('refresh-1');
    session.account.revoked = true;
    const result = await request('GET', '/api/test_auth', { headers: pinned(session.cookie, session.account) });
    session.account.revoked = false;
    assert.equal(result.status, 401);
});

test('chat enforces the selected group and model', async () => {
    const session = await bootstrap('refresh-1');
    const common = { ...pinned(session.cookie, session.account), 'Content-Type': 'application/json' };
    const blockedGroup = await request('POST', '/api/new-api/chat/completions', {
        headers: { ...common, 'X-New-API-Group': 'missing', 'X-New-API-Model': 'default-model' },
        body: Buffer.from(JSON.stringify({ messages: [] })),
    });
    const blockedModel = await request('POST', '/api/new-api/chat/completions', {
        headers: { ...common, 'X-New-API-Group': 'default', 'X-New-API-Model': 'premium-model' },
        body: Buffer.from(JSON.stringify({ messages: [] })),
    });
    assert.equal(blockedGroup.status, 403);
    assert.equal(blockedModel.status, 403);
});

test('catalogue follows the portal name and replaces stale image-only selections', async () => {
    const session = await bootstrap('refresh-1');
    const result = await request('GET', '/api/new-api/account?group=images', {
        headers: pinned(session.cookie, session.account),
    });
    assert.equal(result.status, 200);
    const data = json(result);
    assert.equal(data.systemName, 'Fixture Portal');
    assert.deepEqual(Object.keys(data.groups), ['default', 'premium']);
    assert.equal(data.group, 'default');
    assert.deepEqual(data.models, ['default-model']);
});

test('image-only and Auto-only groups cannot reach paid chat relay', async () => {
    const session = await bootstrap('refresh-1');
    const beforeDebit = debits.length;
    for (const group of ['images', 'auto-only']) {
        const result = await request('POST', '/api/new-api/chat/completions', {
            headers: {
                ...pinned(session.cookie, session.account), 'Content-Type': 'application/json',
                'X-New-API-Group': group, 'X-New-API-Model': 'Auto',
            },
            body: Buffer.from(JSON.stringify({ messages: [] })),
        });
        assert.equal(result.status, 403);
    }
    assert.equal(debits.length, beforeDebit);
});

test('an account with no chat groups receives an empty catalogue', async () => {
    const session = await bootstrap('refresh-1');
    session.account.noChat = true;
    try {
        const result = await request('GET', '/api/new-api/account?group=images', {
            headers: pinned(session.cookie, session.account),
        });
        assert.equal(result.status, 200);
        assert.deepEqual(json(result).groups, {});
        assert.deepEqual(json(result).models, []);
        assert.equal(json(result).group, '');
    } finally {
        session.account.noChat = false;
    }
});

test('streaming completion performs one synthetic debit with enforced routing', async () => {
    const session = await bootstrap('refresh-1');
    const beforeDebit = debits.length;
    const result = await request('POST', '/api/new-api/chat/completions', {
        headers: {
            ...pinned(session.cookie, session.account),
            'X-New-API-Group': 'premium',
            'X-New-API-Model': 'premium-model',
            'Content-Type': 'application/json',
        },
        body: Buffer.from(JSON.stringify({ messages: [{ role: 'user', content: 'hello' }], model: 'ignored', n: 9, unsafe: true, stream: true })),
    });
    assert.equal(result.status, 200);
    assert.match(result.body.toString(), /data: \[DONE\]/);
    assert.equal(debits.length, beforeDebit + 1);
    assert.deepEqual(debits.at(-1).body, {
        messages: [{ role: 'user', content: 'hello' }], stream: true, model: 'premium-model', group: 'premium', n: 1,
    });
    const account = await request('GET', '/api/new-api/account', { headers: pinned(session.cookie, session.account) });
    assert.equal(account.status, 200);
    assert.equal(json(account).account.quota, 10);
    assert.equal(json(account).tavernCredit.quota, 499999);
    assert.ok(!account.body.includes(tavernKey));
});

test('repeat bootstrap and cleared browser storage cannot refill the gift', async () => {
    const first = await bootstrap('refresh-2');
    gifts.get(first.account.id).quota = 123456;
    const second = await bootstrap('refresh-2');
    const result = await request('GET', '/api/new-api/account', {
        headers: { ...pinned(second.cookie, second.account), 'X-Tavern-Service-Key': 'forged-browser-key' },
    });
    assert.equal(result.status, 200);
    assert.equal(json(result).tavernCredit.quota, 123456);
    assert.equal(json(result).account.quota, 10);
    assert.ok(!result.body.includes(tavernKey));
});

test('native proxy and password routes are unavailable in New API mode', async () => {
    for (const [method, pathname] of [['GET', '/api/proxy'], ['POST', '/api/user/password']]) {
        const result = await request(method, pathname);
        assert.equal(result.status, 404);
        assert.equal(json(result).message, 'This endpoint is unavailable in New API mode');
    }
});

test('logout requires the current tab pin', async () => {
    const session = await bootstrap('refresh-1');
    const result = await request('POST', '/api/user/auth/new-api/logout', {
        headers: { Cookie: `${session.cookie}; new_api_refresh=refresh-1`, 'X-New-API-User': session.account.id, 'X-New-API-Session': 'stale' },
    });
    assert.equal(result.status, 409);
});

test('same-origin logout forwards the refresh clear-cookie and clears the bridge session', async () => {
    const session = await bootstrap('refresh-1');
    const result = await request('POST', '/api/user/auth/new-api/logout', {
        headers: { ...pinned(session.cookie, session.account), Cookie: `${session.cookie}; new_api_refresh=refresh-1` },
    });
    assert.equal(result.status, 200);
    const cookies = result.headers['set-cookie'] || [];
    assert.ok(cookies.some(cookie => cookie.startsWith('new_api_refresh=; Max-Age=0')));
    assert.ok(cookies.some(cookie => cookie.startsWith('risu_new_api=;')));
});
