const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const upstream = path.resolve(__dirname, '../../../agnai')
const ts = require(path.join(upstream, 'node_modules/typescript'))
const signals = require(path.join(upstream, 'node_modules/solid-js'))
const compiled = ts.transpileModule(fs.readFileSync(path.join(upstream, 'web/new-api.ts'), 'utf8'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
}).outputText

function client(fetcher, storage = new Map()) {
  let time = 1000000
  const cookies = new Map()
  const exports = {}
  class Clock extends Date { static now() { return time } }
  vm.runInNewContext(compiled, {
    exports,
    require: (name) => name === 'solid-js' ? signals : {
      get: (key) => cookies.get(key), set: (key, value) => cookies.set(key, value), remove: (key) => cookies.delete(key),
    },
    process: { env: { NEW_API_BRIDGE: '1', NEW_API_PORTAL_URL: 'http://localhost:3107' } },
    fetch: fetcher,
    Date: Clock, Intl, URLSearchParams,
    localStorage: { getItem: (key) => storage.get(key), setItem: (key, value) => storage.set(key, value) },
    window: { location: { assign() {} } },
  })
  return { api: exports, cookies, advance: (milliseconds) => { time += milliseconds } }
}
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data })
const login = (id = 'alice') => response({ token: `native-${id}`, user: { _id: id }, accessExpiresAt: 1900 })
const account = (group, model, quota = 5000000) => ({
  account: { id: 901, username: 'fixture', quota }, group,
  groups: { default: { desc: 'Standard' }, premium: { desc: 'Premium' } },
  models: [model], quotaPerUnit: 500000, quotaDisplayType: 'USD',
})

test('native login is shared, renewed before expiry, and upstream bearer never becomes a client header', async () => {
  let calls = 0
  const { api, cookies, advance } = client(async () => { calls++; return login() })
  await api.ensureNewApiSession()
  await api.ensureNewApiSession()
  assert.equal(calls, 1)
  assert.equal(cookies.get('auth'), 'native-alice')
  api.selectNewApiModel('premium', 'premium-model')
  assert.equal(api.newApiHeaders()['X-New-API-Group'], 'premium')
  assert.equal(api.newApiHeaders().Authorization, undefined)
  advance(601000)
  await api.ensureNewApiSession()
  assert.equal(calls, 2)
})

test('account switching during renewal cannot overwrite the previous native identity', async () => {
  let id = 'alice'
  const { api, cookies, advance } = client(async () => login(id))
  await api.ensureNewApiSession()
  id = 'bob'
  advance(601000)
  await assert.rejects(api.ensureNewApiSession(), /account changed/)
  assert.equal(cookies.get('auth'), 'native-alice')
})

test('failed sign-in never installs a native token', async () => {
  const { api, cookies } = client(async () => response({ message: 'Revoked' }, 401))
  await assert.rejects(api.ensureNewApiSession(), /Revoked/)
  assert.equal(cookies.size, 0)
})

test('late group replies cannot replace a newer selection or balance', async () => {
  let finishFirst
  const { api } = client(async (url) => {
    if (url.includes('/auth/')) return login()
    if (url.includes('group=default')) return new Promise((resolve) => { finishFirst = resolve })
    return response(account('premium', 'premium-model', 4900000))
  })
  await api.ensureNewApiSession()
  const first = api.refreshNewApiAccount('default')
  await Promise.resolve()
  const second = api.refreshNewApiAccount('premium')
  await second
  finishFirst(response(account('default', 'old-model')))
  await first
  assert.equal(api.newApiSelection().group, 'premium')
  assert.equal(api.newApiAccount().account.quota, 4900000)
})

test('server balance and revoked model list replace cached values without a client debit ledger', async () => {
  let data = account('default', 'old-model')
  const { api } = client(async (url) => url.includes('/auth/') ? login() : response(data))
  await api.refreshNewApiAccount('default')
  assert.equal(api.newApiSelection().model, 'old-model')
  data = account('default', 'new-model', 6000000)
  await api.refreshNewApiAccount('default')
  assert.equal(api.newApiSelection().model, 'new-model')
  assert.equal(api.newApiAccount().account.quota, 6000000)
  assert.match(api.formatNewApiBalance(data), /12\.00/)
})

test('a revoked cached group returns to the current authorized default', async () => {
  const { api } = client(async (url) => {
    if (url.includes('/auth/')) return login()
    if (url.includes('group=removed')) return response({ message: 'Forbidden' }, 403)
    return response(account('default', 'chat-model'))
  })
  await api.refreshNewApiAccount('removed')
  assert.equal(api.newApiSelection().group, 'default')
  assert.equal(api.newApiSelection().model, 'chat-model')
})

test('reloading restores each account model preference without sharing it with another account', async () => {
  const storage = new Map()
  const fetchAlice = async (url) => url.includes('/auth/')
    ? login('alice')
    : response({ ...account('premium', 'first-model'), models: ['first-model', 'chosen-model'] })
  const first = client(fetchAlice, storage).api
  await first.refreshNewApiAccount('premium')
  first.selectNewApiModel('premium', 'chosen-model')
  const reloaded = client(fetchAlice, storage).api
  await reloaded.refreshNewApiAccount('premium')
  assert.equal(reloaded.newApiSelection().model, 'chosen-model')
  const bob = client(async (url) => url.includes('/auth/')
    ? login('bob')
    : response(account('default', 'bob-model')), storage).api
  await bob.refreshNewApiAccount()
  assert.equal(bob.newApiSelection().model, 'bob-model')
  assert.equal(reloaded.newApiSelection().model, 'chosen-model')
})
