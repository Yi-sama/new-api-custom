const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join } = require('node:path')
const { test } = require('node:test')
const { execFileSync } = require('node:child_process')

const directory = __dirname
const helperPath = join(directory, 'studio-tool-result.js')

async function loadHelper() {
  const source = readFileSync(helperPath, 'utf8')
  return import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`)
}

test('generate_image hides empty optional dimensions without adding result metadata to input', async () => {
  const { formatToolArguments } = await loadHelper()
  const argumentsValue = JSON.stringify({ prompt: 'fixture', size: '', aspect_ratio: '', n: 1 })

  assert.deepEqual(JSON.parse(formatToolArguments('generate_image', argumentsValue)), {
    prompt: 'fixture',
    n: 1,
  })
  assert.equal(JSON.parse(argumentsValue).size, '')
})

test('generate_image keeps non-empty LLM dimensions', async () => {
  const { formatToolArguments } = await loadHelper()

  assert.deepEqual(JSON.parse(formatToolArguments('generate_image', {
    prompt: 'fixture', size: '1024x1024', aspect_ratio: '1:1',
  })), {
    prompt: 'fixture',
    size: '1024x1024',
    aspect_ratio: '1:1',
  })
})

test('generate_image output prepends persisted actual request and preserves original success JSON', async () => {
  const { formatToolOutput } = await loadHelper()
  const output = JSON.stringify({
    status: 'success',
    images: ['unchanged'],
    image_request: { model: 'gpt-image-2', size: '864x1536', n: 1, stage: 'complete' },
  })

  assert.equal(formatToolOutput('generate_image', output),
    `Actual request: gpt-image-2 | 864x1536\n\n${output}`)
})

test('generate_image output supports failed double-encoded JSON and preserves the error payload', async () => {
  const { formatToolOutput } = await loadHelper()
  const output = JSON.stringify(JSON.stringify({
    error: 'download failed',
    image_request: { model: 'gpt-image-2', size: '1024x1024', n: 1, stage: 'downloading' },
  }))

  assert.equal(formatToolOutput('generate_image', output),
    `Actual request: gpt-image-2 | 1024x1024\n\n${output}`)
})

test('other tools and malformed generate_image arguments retain native stringification', async () => {
  const { formatToolArguments, formatToolOutput } = await loadHelper()
  assert.equal(formatToolArguments('search', { query: 'fixture' }, '{}'), '{"query":"fixture"}')
  assert.equal(formatToolArguments('generate_image', '{broken', '{}'), '{broken')
  assert.equal(formatToolOutput('search', '{"result":true}'), '{"result":true}')
  assert.equal(formatToolOutput('generate_image', '{broken'), '{broken')
})

test('frontend patch routes only the display arguments through the helper', () => {
  const fixture = join(directory, '../../.local-tests/studio-native-v15/chunks/Cizwf2GR.js')
  const patched = execFileSync('python', ['-B', '-c',
    'import sys; from pathlib import Path; sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_tool_results; print(patch_tool_results(Path(sys.argv[2]).read_text(encoding="utf-8")))',
    directory, fixture,
  ], { encoding: 'utf8', maxBuffer: 2 * 1024 * 1024 })

  assert.match(patched, /studio-tool-result\.js\?v=23/)
  assert.match(patched, /arguments:NewApiToolArguments\(n\.name,n\.arguments\?\?""\)/)
  assert.match(patched, /text:NewApiToolOutput\(n\.name,O\(s\)\)/)
  assert.match(patched, /status:a/)
  assert.equal((patched.match(/NewApiToolArguments\(/g) || []).length, 1)
  assert.equal((patched.match(/NewApiToolOutput\(/g) || []).length, 1)
})

test('image studio build entry and injected helpers use the v23 cache namespace', () => {
  const patchSource = readFileSync(join(directory, 'patch_frontend.py'), 'utf8')
  const dockerfile = readFileSync(join(directory, 'Dockerfile'), 'utf8')

  assert.match(patchSource, /build \/ "studio-v23"/)
  assert.doesNotMatch(patchSource, /\/static\/studio-[^"']+\?v=2[12]/)
  assert.match(dockerfile, /studio-tool-result\.js \/app\/build\/static\//)
  assert.match(dockerfile, /studio-link\.js\?v=24/)
  assert.match(dockerfile, /billing-link\.js\?v=24/)
})
