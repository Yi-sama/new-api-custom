const { createCipheriv, createDecipheriv, createHash, randomBytes, randomUUID } = require('node:crypto');
const fs = require('node:fs/promises');
const { mkdirSync, readFileSync, writeFileSync } = require('node:fs');
const path = require('node:path');
const { Readable } = require('node:stream');
const { pipeline } = require('node:stream/promises');

const cookieName = 'risu_new_api';
const chatFields = new Set([
    'messages', 'temperature', 'top_p', 'max_tokens', 'max_completion_tokens',
    'presence_penalty', 'frequency_penalty', 'logit_bias', 'stop', 'seed',
    'response_format', 'tools', 'tool_choice', 'parallel_tool_calls',
    'reasoning_effort', 'verbosity', 'stream', 'stream_options',
]);

function failure(message, status = 400) {
    return Object.assign(new Error(message), { status });
}

function httpUrl(value, allowPublicHttp = false) {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password
        || url.pathname !== '/' || url.search || url.hash) {
        throw new Error('The New API bridge requires an HTTP(S) origin without credentials or a path');
    }
    if (url.protocol === 'http:' && !allowPublicHttp && !['localhost', '127.0.0.1'].includes(url.hostname)) {
        throw new Error('The New API bridge is restricted to HTTP loopback origins');
    }
    return url;
}

function localUrl(value) {
    const url = httpUrl(value);
    if (url.protocol !== 'http:' || !['localhost', '127.0.0.1'].includes(url.hostname)) {
        throw new Error('The New API backend is restricted to HTTP loopback origins');
    }
    return url.origin;
}

function cookie(req, name) {
    return (req.get('cookie') || '').split(';').map(v => v.trim())
        .find(v => v.startsWith(`${name}=`))?.slice(name.length + 1);
}

function userId(account) {
    const value = String(account?.id || '');
    if (!/^[1-9][0-9]*$/.test(value) || !Number.isSafeInteger(Number(value))) {
        throw failure('Invalid New API account', 401);
    }
    return value;
}

function assertAccount(account, expectedId) {
    if (userId(account) !== expectedId || account.status !== 1 || account.disabled
        || account.revoked || account.active === false) {
        throw failure('New API sign-in required', 401);
    }
}

function storageKey(value) {
    if (typeof value !== 'string' || !/^(?:[0-9a-fA-F]{2}){1,120}$/.test(value)) {
        throw failure('Invalid storage key');
    }
    return value.toLowerCase();
}

function installNewApi(app, options = {}) {
    const allowPublicHttp = options.allowPublicHttp === undefined
        ? process.env.NEW_API_ALLOW_PUBLIC_HTTP === '1'
        : options.allowPublicHttp === true;
    const backend = localUrl(options.backendUrl || process.env.NEW_API_URL);
    const portalUrl = httpUrl(options.portalUrl || process.env.NEW_API_PORTAL_URL, allowPublicHttp);
    const publicUrl = httpUrl(options.publicUrl || process.env.NEW_API_PUBLIC_URL, allowPublicHttp);
    if (portalUrl.hostname !== publicUrl.hostname) {
        throw new Error('The New API portal and public origins must use the same hostname');
    }
    if (portalUrl.protocol !== publicUrl.protocol) {
        throw new Error('The New API portal and public origins must use the same protocol');
    }
    const portal = portalUrl.origin;
    const publicOrigin = publicUrl.origin;
    const dataDir = path.toNamespacedPath(path.resolve(options.dataDir || process.env.NEW_API_DATA_DIR));
    const request = options.request || fetch;
    const tavernKeyFile = process.env.TAVERN_SERVICE_KEY_FILE;
    const tavernKey = options.tavernKey || (tavernKeyFile ? readFileSync(tavernKeyFile, 'utf8').trim() : '');
    if (tavernKey && tavernKey.length < 32) throw new Error('Invalid tavern service key');
    const tavernHeaders = () => ({ 'X-Tavern-Service-Key': tavernKey });
    const sessionKey = (() => {
        mkdirSync(dataDir, { recursive: true });
        if (options.secret) return createHash('sha256').update(options.secret).digest();
        const secretPath = path.join(dataDir, '.session-secret');
        try {
            writeFileSync(secretPath, randomBytes(32), { flag: 'wx', mode: 0o600 });
        } catch (error) {
            if (error.code !== 'EEXIST') throw error;
        }
        return createHash('sha256').update(readFileSync(secretPath)).digest();
    })();

    async function openSession(req) {
        const encoded = cookie(req, cookieName);
        if (!encoded) return undefined;
        try {
            const bytes = Buffer.from(encoded, 'base64url');
            if (bytes.length < 29) return undefined;
            const decipher = createDecipheriv('aes-256-gcm', sessionKey, bytes.subarray(0, 12));
            decipher.setAuthTag(bytes.subarray(12, 28));
            const session = JSON.parse(Buffer.concat([
                decipher.update(bytes.subarray(28)), decipher.final(),
            ]).toString('utf8'));
            if (!session.sid || typeof session.accessToken !== 'string'
                || !Number.isFinite(session.accessExpiresAt)) return undefined;
            userId({ id: session.userId });
            return session;
        } catch {
            return undefined;
        }
    }

    async function writeSession(res, session) {
        const iv = randomBytes(12);
        const cipher = createCipheriv('aes-256-gcm', sessionKey, iv);
        const encrypted = Buffer.concat([
            cipher.update(JSON.stringify(session), 'utf8'), cipher.final(),
        ]);
        const value = Buffer.concat([iv, cipher.getAuthTag(), encrypted]).toString('base64url');
        res.cookie(cookieName, value, {
            httpOnly: true, secure: publicUrl.protocol === 'https:', sameSite: 'strict', path: '/', maxAge: 86400000,
        });
    }

    function pin(req, session) {
        if (!session || req.get('X-New-API-User') !== session.userId
            || req.get('X-New-API-Session') !== session.sid) {
            throw failure('New API account changed. Reopen Tavern from New API.', 409);
        }
    }

    async function upstream(route, session, init = {}) {
        let response;
        try {
            response = await request(`${backend}/${route}`, {
                ...init, redirect: 'error', signal: init.signal || AbortSignal.timeout(15000),
                headers: {
                    ...(session ? { Authorization: `Bearer ${session.accessToken}` } : {}),
                    ...init.headers,
                },
            });
        } catch {
            throw failure('New API is unavailable', 503);
        }
        return response;
    }

    async function api(route, session, init = {}) {
        const response = await upstream(route, session, init);
        const body = await response.json().catch(() => null);
        if (!response.ok || !body?.success) {
            throw failure('Unable to read New API account',
                [401, 403].includes(response.status) ? 401 : 502);
        }
        return body.data;
    }

    async function authenticate(req) {
        const session = await openSession(req);
        if (!session || session.accessExpiresAt <= Date.now() / 1000) {
            throw failure('New API sign-in required', 401);
        }
        pin(req, session);
        const account = await api('api/user/self', session);
        assertAccount(account, session.userId);
        return { session, account };
    }

    async function catalogue(session, account, requestedGroup, strict = false) {
        const available = await api('api/user/self/groups', session);
        if (requestedGroup && (typeof requestedGroup !== 'string' || requestedGroup.length > 256
            || !Object.hasOwn(available || {}, requestedGroup))) {
            throw failure('Group is not available to this account', 403);
        }
        const entries = await Promise.all(Object.entries(available || {}).map(async ([group, details]) => {
            const models = await api(`api/user/models?${new URLSearchParams({
                group, modality: 'chat',
            })}`, session);
            return [group, details, Array.isArray(models) ? models.filter(v => typeof v === 'string' && v !== 'Auto') : []];
        }));
        const chatEntries = entries.filter(([, , models]) => models.length > 0);
        const groups = Object.fromEntries(chatEntries.map(([group, details]) => [group, details]));
        if (strict && !Object.hasOwn(groups, requestedGroup)) {
            throw failure('No chat models are available in this group', 403);
        }
        const group = [requestedGroup, account.group, chatEntries[0]?.[0]]
            .find(value => value && Object.hasOwn(groups, value)) || '';
        return { groups, group, models: chatEntries.find(([key]) => key === group)?.[2] || [] };
    }

    function forwardCookies(res, response) {
        for (const value of response.headers.getSetCookie()) res.append('Set-Cookie', value);
    }

    function handle(callback) {
        return (req, res, next) => Promise.resolve(callback(req, res, next)).catch(error => {
            if (res.headersSent) {
                res.destroy();
                return;
            }
            const status = error.status || 500;
            const message = status === 500 ? 'RisuAI account operation failed' : error.message;
            res.status(status).json({ message, error: { message } });
        });
    }

    app.use((req, res, next) => {
        if (!/^\/(?:api(?:\/|$)|proxy|hub-proxy)/.test(req.path)) return next();
        res.set('Cache-Control', 'no-store');
        if (req.get('host') !== publicUrl.host
            || (req.method !== 'GET' && req.method !== 'HEAD' && req.get('origin') !== publicOrigin)
            || req.get('sec-fetch-site') === 'cross-site') {
            return res.status(403).json({ message: 'Forbidden origin' });
        }
        next();
    });

    app.post('/api/user/auth/new-api', handle(async (req, res) => {
        const refresh = cookie(req, 'new_api_refresh');
        if (!refresh) throw failure('Sign in to New API first', 401);
        const previous = await openSession(req);
        const expected = req.body?.reconnect === true ? undefined : previous;
        const response = await upstream('api/user/auth/refresh', undefined, {
            method: 'POST',
            headers: {
                Cookie: `new_api_refresh=${refresh}`,
                ...(expected ? { 'X-Auth-Session': expected.sid } : {}),
            },
        });
        forwardCookies(res, response);
        const body = await response.json().catch(() => null);
        const data = body?.data;
        if (!response.ok || !body?.success || typeof data?.access_token !== 'string'
            || !data.session?.sid || !(data.access_expires_at > Date.now() / 1000)) {
            throw failure('New API sign-in required', 401);
        }
        const session = {
            userId: userId(data.user), sid: data.session.sid,
            accessToken: data.access_token, accessExpiresAt: data.access_expires_at,
        };
        if (expected && (expected.userId !== session.userId || expected.sid !== session.sid)) {
            throw failure('New API account changed. Reopen Tavern.', 409);
        }
        assertAccount(await api('api/user/self', session), session.userId);
        if (tavernKey) {
            await api('pg/tavern/credit', session, { method: 'POST', headers: tavernHeaders() });
        }
        await writeSession(res, session);
        res.json({
            userId: session.userId, sid: session.sid,
            accessExpiresAt: session.accessExpiresAt, portalUrl: portal,
        });
    }));

    app.post('/api/user/auth/new-api/logout', handle(async (req, res) => {
        const session = await openSession(req);
        pin(req, session);
        const refresh = cookie(req, 'new_api_refresh');
        if (!refresh) throw failure('New API sign-in required', 401);
        const response = await upstream('api/user/auth/logout', session, {
            method: 'POST',
            headers: { Cookie: `new_api_refresh=${refresh}`, 'X-Auth-Session': session.sid },
        });
        const result = await response.json().catch(() => null);
        if (!response.ok || !result?.success) {
            throw failure('Unable to sign out of New API', response.ok ? 502 : response.status);
        }
        forwardCookies(res, response);
        res.clearCookie(cookieName, { httpOnly: true, secure: publicUrl.protocol === 'https:', sameSite: 'strict', path: '/' });
        res.json({ success: true });
    }));

    app.get('/api/new-api/account', handle(async (req, res) => {
        const { session, account } = await authenticate(req);
        const result = await catalogue(session, account,
            typeof req.query.group === 'string' ? req.query.group : undefined);
        const status = await api('api/status', session);
        const tavernCredit = tavernKey
            ? await api('pg/tavern/credit', session, { headers: tavernHeaders() })
            : undefined;
        res.json({
            ...result,
            account: {
                id: account.id, username: account.username,
                display_name: account.display_name, quota: account.quota, group: account.group,
            },
            quotaPerUnit: status.quota_per_unit, quotaDisplayType: status.quota_display_type,
            usdExchangeRate: status.usd_exchange_rate,
            customCurrencySymbol: status.custom_currency_symbol,
            customCurrencyExchangeRate: status.custom_currency_exchange_rate,
            systemName: status.system_name,
            tavernCredit,
        });
    }));

    app.get('/api/test_auth', handle(async (req, res) => {
        await authenticate(req);
        res.json({ status: 'success' });
    }));

    app.use(['/api/read', '/api/write', '/api/list', '/api/remove'], handle(async (req, res) => {
        const route = req.baseUrl;
        if (req.path !== '/' || req.method !== (['/api/write', '/api/remove'].includes(route) ? 'POST' : 'GET')) {
            throw failure('Method not allowed', 405);
        }
        const { session } = await authenticate(req);
        const folder = path.join(dataDir, 'users', session.userId);
        await fs.mkdir(folder, { recursive: true });
        if (route === '/api/list') {
            const files = (await fs.readdir(folder)).filter(v => /^(?:[a-f0-9]{2})+$/.test(v));
            return res.json({ success: true, content: files.map(v => Buffer.from(v, 'hex').toString('utf8')) });
        }
        if (route === '/api/remove') {
            const encoded = req.get('file-path');
            if (typeof encoded !== 'string' || !/^(?:[0-9a-fA-F]{2})+$/.test(encoded)) {
                throw failure('Invalid storage key');
            }
            const keys = Buffer.from(encoded, 'hex').toString('utf8').split('$$')
                .map(v => storageKey(Buffer.from(v, 'utf8').toString('hex')));
            for (const item of keys) await fs.rm(path.join(folder, item), { force: true });
            return res.json({ success: true });
        }
        const key = storageKey(req.get('file-path'));
        if (route === '/api/read') {
            try {
                const data = await fs.readFile(path.join(folder, key));
                res.type('application/octet-stream').send(data);
            } catch (error) {
                if (error.code !== 'ENOENT') throw error;
                res.type('application/octet-stream').send(Buffer.alloc(0));
            }
            return;
        }
        if (route === '/api/write') {
            if (!Buffer.isBuffer(req.body)) throw failure('Binary storage payload required');
            const temporary = path.join(folder, `.${randomUUID()}.tmp`);
            try {
                await fs.writeFile(temporary, req.body);
                await fs.rename(temporary, path.join(folder, key));
            } finally {
                await fs.rm(temporary, { force: true });
            }
            return res.json({ success: true });
        }
    }));

    app.post('/api/new-api/chat/completions', handle(async (req, res) => {
        const { session, account } = await authenticate(req);
        const group = req.get('X-New-API-Group');
        const model = req.get('X-New-API-Model');
        if (!group || !model || !req.body || !Array.isArray(req.body.messages)) {
            throw failure('Select an available group and model first');
        }
        const available = await catalogue(session, account, group, true);
        if (!available.models.includes(model)) throw failure('Model is not available', 403);
        const body = Object.fromEntries(Object.entries(req.body).filter(([key]) => chatFields.has(key)));
        Object.assign(body, { model, group, n: 1 });
        const abort = new AbortController();
        const closed = () => { if (!res.writableEnded) abort.abort(); };
        res.on('close', closed);
        const timer = setTimeout(() => abort.abort(), 300000);
        try {
            const response = await upstream('pg/chat/completions', session, {
                method: 'POST', headers: {
                    'Content-Type': 'application/json',
                    ...(tavernKey ? tavernHeaders() : {}),
                },
                body: JSON.stringify(body), signal: abort.signal,
            });
            res.status(response.status);
            res.set('Content-Type', response.headers.get('content-type') || 'application/json');
            res.set('X-Accel-Buffering', 'no');
            if (response.body) await pipeline(Readable.fromWeb(response.body), res);
            else res.end();
        } finally {
            clearTimeout(timer);
            res.off('close', closed);
        }
    }));

    app.use((req, res, next) => {
        if (/^\/(?:api(?:\/|$)|proxy|hub-proxy)/.test(req.path)) {
            return res.status(404).json({ message: 'This endpoint is unavailable in New API mode' });
        }
        next();
    });
}

module.exports = { installNewApi };
