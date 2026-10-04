const { createRequire } = require('node:module');
const { mkdirSync, readFileSync, writeFileSync, copyFileSync } = require('node:fs');
const path = require('node:path');

async function main() {
    const [source, destination] = process.argv.slice(2).map(value => path.resolve(value));
    const sourceRequire = createRequire(path.join(source, 'package.json'));
    const viteRequire = createRequire(sourceRequire.resolve('vite'));
    const esbuild = viteRequire('esbuild');
    mkdirSync(destination, { recursive: true });
    const result = await esbuild.build({
        entryPoints: [path.join(source, 'server/node/server.cjs')],
        outfile: path.join(destination, 'server.cjs'),
        bundle: true,
        platform: 'node',
        target: 'node22',
        format: 'cjs',
        external: ['bufferutil', 'utf-8-validate'],
        legalComments: 'linked',
        metafile: true,
    });
    writeFileSync(path.join(destination, 'bundle-meta.json'), JSON.stringify(result.metafile, null, 2), 'utf8');
    const packages = new Set();
    for (const input of Object.keys(result.metafile.inputs)) {
        let directory = path.dirname(path.resolve(input));
        while (directory !== path.dirname(directory)) {
            try {
                const file = path.join(directory, 'package.json');
                const manifest = JSON.parse(readFileSync(file, 'utf8'));
                if (manifest.name && directory.includes('node_modules')) packages.add(directory);
                break;
            } catch {
                directory = path.dirname(directory);
            }
        }
    }
    for (const directory of packages) {
        const manifest = JSON.parse(readFileSync(path.join(directory, 'package.json'), 'utf8'));
        const target = path.join(destination, 'licenses', `${manifest.name.replaceAll('/', '__')}@${manifest.version}`);
        mkdirSync(target, { recursive: true });
        copyFileSync(path.join(directory, 'package.json'), path.join(target, 'package.json'));
        for (const name of ['LICENSE', 'LICENSE.md', 'LICENSE.txt', 'license', 'NOTICE']) {
            try {
                copyFileSync(path.join(directory, name), path.join(target, name));
            } catch (error) {
                if (error.code !== 'ENOENT') throw error;
            }
        }
    }
}

main().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
