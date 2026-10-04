const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');

const root = __dirname;
const source = path.join(root, 'source', 'risuai');
const runtime = path.join(root, 'release', 'risuai-managed');
const resultFile = path.join(root, 'tavern-build-result.json');
const temporary = path.join(root, `tavern-dist-${Date.now()}`);
const log = fs.openSync(path.join(root, 'logs', 'tavern-deferred-build.log'), 'w');
try {
  if (fs.existsSync(path.join(runtime, 'dist'))) throw new Error('A Tavern frontend already exists; do not overwrite it.');
  console.log(`BUILD_PID=${process.pid}`);
  const result = spawnSync(process.execPath, [
    path.join(source, 'node_modules', 'vite', 'bin', 'vite.js'),
    'build', '--base', '/tavern-app/', '--minify', 'false', '--outDir', temporary,
  ], {
    cwd: source,
    env: {
      ...process.env,
      NODE_OPTIONS: '--max-old-space-size=4096',
      VITE_NEW_API_BRIDGE: '1',
      VITE_NEW_API_PORTAL_URL: 'https://api.yisama.asia',
      VITE_RISU_LEGAL_CONFIGURED: '',
    },
    stdio: ['ignore', log, log],
    timeout: 240000,
    windowsHide: true,
  });
  if (result.error || result.status !== 0) throw result.error || new Error(`Vite exited ${result.status}`);
  const html = fs.readFileSync(path.join(temporary, 'index.html'), 'utf8');
  if (!html.includes('/tavern-app/assets/')) throw new Error('Incorrect production asset base.');
  fs.renameSync(temporary, path.join(runtime, 'dist'));
  fs.copyFileSync(path.join(source, 'LICENSE'), path.join(runtime, 'LICENSE.risuai'));
  const hash = crypto.createHash('sha256').update(fs.readFileSync(path.join(runtime, 'server.cjs'))).digest('hex');
  const metadata = {
    status: 'pass', legalConfigured: false, nativeLegalNoticeRetained: true,
    portal: 'https://api.yisama.asia', base: '/tavern-app/',
    runtimeSha256: hash, paidCalls: 0,
  };
  fs.writeFileSync(resultFile, JSON.stringify(metadata, null, 2) + '\n');
  console.log(JSON.stringify(metadata));
} catch (error) {
  fs.writeFileSync(resultFile, JSON.stringify({ status: 'fail', message: error.message }) + '\n');
  console.error(error.message);
  process.exitCode = 1;
} finally {
  fs.closeSync(log);
}
