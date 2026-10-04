const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');
const root = __dirname;
const release = path.join(root, 'release');
const build = JSON.parse(fs.readFileSync(path.join(root, 'tavern-build-result.json'), 'utf8'));
if (build.status !== 'pass' || build.legalConfigured !== false) throw new Error('Deferred-notice build not verified.');
const manifestFile = path.join(release, 'candidate-manifest.json');
const manifest = JSON.parse(fs.readFileSync(manifestFile, 'utf8'));
const sha256 = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const browserLog = fs.readFileSync(path.join(root, 'logs/candidate-browser.log'), 'utf8');
const browser = JSON.parse(browserLog.split(/\r?\n/).find(line => line.startsWith('{"results":')));
if (browser.results.length !== 2 || browser.results.some(result => result.errors.length)
    || browser.generationRequests !== 0) throw new Error('Candidate browser checks failed.');
const inventoryFile = path.join(root, 'source-inventory.json');
const inventory = JSON.parse(fs.readFileSync(inventoryFile, 'utf8'));
const layoutPath = 'web/src/components/layout/components/system-brand.tsx';
for (const record of inventory) {
  const actual = sha256(path.join(root, 'source/new-api', record.path));
  if (record.path === layoutPath) record.sha256 = actual;
  else if (actual !== record.sha256) throw new Error('Unexpected frozen source change: ' + record.path);
}
fs.writeFileSync(inventoryFile, JSON.stringify(inventory, null, 2) + '\n');
manifest.backend_sha256 = sha256(path.join(release, 'new-api'));
manifest.candidate_visual_verification = 'desktop_1440_and_mobile_390_pass_synthetic_account';
manifest.mobile_header_overflow_fixed = true;
manifest.legal_configuration_deferred = true;
manifest.tavern_production_build = 'built_with_native_unconfigured_notice';
manifest.risu_runtime_bundle_sha256 = build.runtimeSha256;
manifest.status = 'ready_for_deferred_notice_rollout';
fs.writeFileSync(manifestFile, JSON.stringify(manifest, null, 2) + '\n');
const records = [];
function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) throw new Error('Release must not contain dependency links.');
    if (entry.isDirectory()) walk(file);
    else if (entry.name !== 'SHA256SUMS') {
      const hash = crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
      records.push(`${hash}  ${path.relative(release, file).replaceAll('\\', '/')}`);
    }
  }
}
walk(release);
fs.writeFileSync(path.join(release, 'SHA256SUMS'), records.sort().join('\n') + '\n');
const archive = path.join(root, 'production-suite-20261003-001.tgz');
const result = spawnSync('tar', ['-czf', archive, '-C', release, '.'], { stdio: 'inherit', windowsHide: true });
if (result.status !== 0) throw new Error('Release archive failed.');
const hash = crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex');
fs.writeFileSync(path.join(root, 'package-result.json'), JSON.stringify({
  files: records.length, archive, bytes: fs.statSync(archive).size, sha256: hash,
  legalConfigured: false, paidCalls: 0,
}, null, 2) + '\n');
console.log(`PACKAGE_FILES=${records.length} ARCHIVE_SHA256=${hash}`);
