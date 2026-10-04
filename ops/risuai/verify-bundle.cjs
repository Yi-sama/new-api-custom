const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs/promises');
const http = require('node:http');
const path = require('node:path');

function request(route, options = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request('http://127.0.0.1:32831' + route, options, res => {
            let body = '';
            res.setEncoding('utf8');
            res.on('data', data => { body += data; });
            res.on('end', () => resolve({ status: res.statusCode, body }));
        });
        req.on('error', reject);
        req.end(options.body);
    });
}

async function main() {
    const release = path.resolve(process.argv[2]);
    const runtime = await fs.mkdtemp(path.join(release, '..', 'bundle-verification-'));
    await fs.symlink(path.join(release, 'dist'), path.join(runtime, 'dist'), 'junction');
    const child = spawn(process.execPath, [path.join(release, 'server.cjs')], {
        cwd: runtime,
        windowsHide: true,
        env: {
            ...process.env,
            PORT: '32831',
            NEW_API_URL: 'http://127.0.0.1:9',
            NEW_API_PORTAL_URL: 'http://156.238.252.115:32795',
            NEW_API_PUBLIC_URL: 'http://156.238.252.115:32796',
            NEW_API_ALLOW_PUBLIC_HTTP: '1',
            NEW_API_DATA_DIR: path.join(runtime, 'data'),
            TRUST_PROXY: 'loopback',
        },
        stdio: ['ignore', 'pipe', 'pipe'],
    });
    let output = '';
    const finished = new Promise(resolve => child.once('exit', resolve));
    try {
        await new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error('Bundled server startup timed out')), 15000);
            const capture = data => {
                output += data;
                if (output.includes('HTTP server is running')) {
                    clearTimeout(timer);
                    resolve();
                }
            };
            child.stdout.on('data', capture);
            child.stderr.on('data', capture);
            child.once('error', reject);
            child.once('exit', code => {
                clearTimeout(timer);
                reject(new Error(`Bundled server exited (${code}): ${output}`));
            });
        });
        const host = '156.238.252.115:32796';
        const origin = 'http://' + host;
        const root = await request('/', { headers: { Host: host } });
        assert.equal(root.status, 200);
        assert.match(root.body, /globalThis\.__NODE__ = true/);
        for (const [route, headers, status] of [
            ['/api/new-api/account', { Host: host }, 401],
            ['/api/new-api/account', { Host: 'hostile.example' }, 403],
            ['/api/password', { Host: host }, 404],
        ]) {
            const response = await request(route, { headers });
            assert.equal(response.status, status);
        }
        for (const [requestOrigin, status] of [[origin, 401], ['http://hostile.example', 403]]) {
            const response = await request('/api/user/auth/new-api', {
                method: 'POST',
                headers: { Host: host, Origin: requestOrigin, 'Content-Type': 'application/json' },
                body: '{}',
            });
            assert.equal(response.status, status);
        }
        console.log('BUNDLED_RUNTIME=PASS (HTTP root, authentication, Host, Origin, disabled native API)');
    } finally {
        if (child.exitCode === null) child.kill();
        await finished;
    }
}

main().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
