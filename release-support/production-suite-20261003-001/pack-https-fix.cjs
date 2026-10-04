const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');
const root = path.join(__dirname, 'https-fix');
const records = [];
function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) throw new Error('Unexpected fix symlink');
    if (entry.isDirectory()) walk(file);
    else if (entry.name !== 'SHA256SUMS') records.push(
      crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') +
      '  ' + path.relative(root, file).replaceAll('\\', '/'));
  }
}
walk(root);
fs.writeFileSync(path.join(root, 'SHA256SUMS'), records.sort().join('\n') + '\n');
const archive = path.join(__dirname, 'https-fix.tgz');
const result = spawnSync('tar', ['-czf', archive, '-C', root, '.'], { stdio: 'inherit', windowsHide: true });
if (result.status !== 0) throw new Error('Fix archive failed');
console.log('HTTPS_FIX_SHA256=' + crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex'));
