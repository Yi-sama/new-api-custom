const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join } = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const { JSDOM } = require(require.resolve('jsdom', { paths: [join(__dirname, '../../web')] }))
const source = readFileSync(join(__dirname, 'studio-link.js'), 'utf8')
const flush = () => new Promise(setImmediate)
async function setup(t, initial = {}) {
  const dom = new JSDOM('<nav><button id="model-selector-model-button">Model</button></nav><button id="schedule">Schedule</button>', { url: 'http://localhost:3002/' })
  t.after(() => dom.window.close())
  const state = {
    identity: { id: 4, username: 'Alice', email: 'new-api-4@studio.example.com', group: 'default' },
    contextStatus: 200,
    modelsStatus: 200,
    nativeStatus: 200,
    models: [{ id: 'gpt-test' }],
    groups: { default: { desc: 'Default' }, '\u5206\u7ec4': { desc: 'Special' } },
    requests: [],
    redirects: [],
    intervals: [],
    resets: 0,
    synced: [],
    imageGroup: '',
    imageModels: [],
    imageError: '',
    ...initial,
  }
  const location = {
    href: 'http://localhost:3002/',
    origin: 'http://localhost:3002',
    pathname: '/',
    replace: (url) => state.redirects.push(url),
  }
  const native = {
    watchUser: (value, onMismatch, onReady) => {
      state.watched = value; state.onMismatch = onMismatch
      onReady()
    },
    syncUser: (value) => state.synced.push(value),
    setModels: (value) => { state.shownModels = value },
    setGroupPicker: (value) => { state.picker = value },
    readImageStudio: async () => state.pendingImages || { group: state.imageGroup, models: state.imageModels, error: state.imageError },
    setImageStudio: (value) => { state.imageStudio = value },
  }
  const window = {
    location,
    setInterval: (fn, delay) => state.intervals.push({ fn, delay }),
    addEventListener: dom.window.addEventListener.bind(dom.window),
    dispatchEvent: dom.window.dispatchEvent.bind(dom.window),
    fetch: async (input, options) => {
      const path = typeof input === 'string' ? input : input.url
      state.requests.push({ path, options })
      if (path === '/api/image-studio/context') return Response.json(state.identity, { status: state.contextStatus })
      if (path === '/api/v1/images/studio/config') return Response.json({ group: state.imageGroup })
      if (path === '/api/image-studio/groups') return state.pendingGroups ||
        Response.json({ success: true, data: state.groups })
      if (path === '/api/models') return state.pendingModels ?
        state.pendingModels : Response.json({ data: state.models }, { status: state.modelsStatus })
      return Response.json({ ok: true }, { status: state.nativeStatus })
    },
  }
  dom.window.addEventListener('new-api-group-changed', () => { state.resets++ })
  const context = vm.createContext({
    window, location, document: dom.window.document,
    localStorage: dom.window.localStorage, sessionStorage: dom.window.sessionStorage,
    Event: dom.window.Event, MutationObserver: dom.window.MutationObserver,
    URL, Request, Response, Headers, console: { error: () => {} },
  })
  const module = new vm.SyntheticModule(Object.keys(native), function () {
    for (const [key, value] of Object.entries(native)) this.setExport(key, value)
  }, { context })
  await module.link(() => {})
  await module.evaluate()
  vm.runInContext(source, context, { importModuleDynamically: async () => module })
  dom.window.document.dispatchEvent(new dom.window.Event('DOMContentLoaded'))
  await flush()
  await flush()
  return { state, window, dom }
}

test('first native request and picker store use the current account and authorized group', async (t) => {
  const { state, window, dom } = await setup(t)
  await window.fetch('/api/models')
  const request = state.requests.findLast(({ path }) => path === '/api/models')
  assert.equal(request.options.headers.get('X-NewAPI-Group'), 'default')
  assert.equal(request.options.headers.get('X-NewAPI-Identity'), state.identity.email)
  assert.equal(state.picker.value, 'default')
  assert.equal(state.picker.items[0].label, 'default (Default)')
  assert.equal(state.picker.disabled, false)
  assert.equal(dom.window.document.querySelector('nav').children.length, 1)
  assert.ok(dom.window.document.querySelector('#schedule'))
  await window.fetch('https://example.com/api/models')
  assert.equal(state.requests.at(-1).options, undefined)
})

test('changing group resets native selection and model store before loading scoped models', async (t) => {
  const { state, dom } = await setup(t)
  dom.window.sessionStorage.setItem('selectedModels', 'old-model')
  const change = state.picker.onChange('\u5206\u7ec4')
  assert.equal(state.resets, 1)
  assert.equal(state.shownModels.length, 0)
  assert.equal(dom.window.sessionStorage.getItem('selectedModels'), null)
  assert.equal(state.picker.disabled, true)
  await change
  assert.equal(state.requests.at(-1).options.headers.get('X-NewAPI-Group'), encodeURIComponent('\u5206\u7ec4'))
  assert.deepEqual(state.shownModels, state.models)
  assert.equal(state.picker.disabled, false)
  assert.equal(state.picker.value, '\u5206\u7ec4')
  assert.equal(dom.window.sessionStorage.getItem('new-api-group:4'), '\u5206\u7ec4')
})

test('picker displays current account ratios, ranges and automatic labels without inventing values', async (t) => {
  const { state } = await setup(t)
  state.groups = {
    default: { desc: 'Default', ratio: 1 },
    vip: { ratio: 0.5, ratio_min: 0.5, ratio_max: 1.5 },
    equal: { ratio: 2, ratio_min: 0.75, ratio_max: 0.75 },
    free: { ratio: 0 },
    auto: { ratio: '\u81ea\u52a8' },
    unknown: {},
  }
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.deepEqual(Array.from(state.picker.items, ({ ratio }) => ratio),
    ['1\u00d7', '0.5-1.5\u00d7', '0.75\u00d7', '0\u00d7', '\u81ea\u52a8', ''])
  state.groups.default.ratio = 1.25
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.equal(state.picker.items[0].ratio, '1.25\u00d7')
  assert.equal(state.picker.items[0].label, 'default (Default)')
  assert.equal(state.picker.value, 'default')
  assert.equal(state.resets, 0)
})

test('failed model fetch keeps previous group models cleared', async (t) => {
  const { state } = await setup(t)
  state.modelsStatus = 503
  await state.picker.onChange('\u5206\u7ec4')
  assert.equal(state.shownModels.length, 0)
  assert.equal(state.picker.disabled, false)
  assert.equal(state.picker.error, true)
})

test('pending group changes cannot overlap and late results cannot survive account switching', async (t) => {
  const { state } = await setup(t)
  let resolve
  state.pendingModels = new Promise((done) => { resolve = done })
  const change = state.picker.onChange('\u5206\u7ec4')
  await state.picker.onChange('default')
  assert.equal(state.picker.value, '\u5206\u7ec4')
  assert.equal(state.resets, 1)
  await flush()
  state.onMismatch()
  resolve(Response.json({ data: [{ id: 'old-account-model' }] }))
  await change
  assert.equal(state.shownModels.length, 0)
  assert.equal(state.picker.disabled, true)
  assert.deepEqual(state.redirects, ['http://localhost:3000/image-studio'])
})

test('an old model request cannot unlock or overwrite a newer permission refresh', async (t) => {
  const { state } = await setup(t)
  let resolveGroups, resolveOld, resolveCurrent
  state.pendingGroups = new Promise((resolve) => { resolveGroups = resolve })
  const refresh = state.intervals.find(({ delay }) => delay === 15000).fn()
  await flush()
  state.pendingModels = new Promise((resolve) => { resolveOld = resolve })
  const oldChange = state.picker.onChange('\u5206\u7ec4')
  await flush()
  state.pendingModels = new Promise((resolve) => { resolveCurrent = resolve })
  resolveGroups(Response.json({ success: true, data: { default: { desc: 'Default' } } }))
  await flush()
  resolveOld(Response.json({ data: [{ id: 'revoked-group-model' }] }))
  await oldChange
  assert.equal(state.picker.value, 'default')
  assert.equal(state.picker.disabled, true)
  assert.equal(state.shownModels.length, 0)
  resolveCurrent(Response.json({ data: [{ id: 'current-group-model' }] }))
  await refresh
  assert.equal(state.picker.disabled, false)
  assert.equal(state.shownModels[0].id, 'current-group-model')
})

test('main-site logout clears native token and returns to the New API entry', async (t) => {
  const { state, dom } = await setup(t)
  dom.window.localStorage.setItem('token', 'old-native-token')
  state.contextStatus = 401
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.equal(dom.window.localStorage.getItem('token'), null)
  assert.deepEqual(state.redirects, ['http://localhost:3000/image-studio'])
})

test('account switch cannot relabel an old native identity', async (t) => {
  const { state } = await setup(t)
  state.identity = { ...state.identity, id: 5, email: 'new-api-5@studio.example.com' }
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.deepEqual(state.redirects, ['http://localhost:3000/image-studio'])
  assert.equal(state.synced.length, 0)
})

test('same-account rename updates native user data without navigating', async (t) => {
  const { state } = await setup(t)
  state.identity = { ...state.identity, username: 'Renamed' }
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.equal(state.synced[0].username, 'Renamed')
  assert.deepEqual(state.redirects, [])
})

test('revoked group is removed and selection is reset to a currently allowed group', async (t) => {
  const { state, dom } = await setup(t)
  state.groups = { '\u5206\u7ec4': { desc: 'Special' } }
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.equal(state.picker.items.length, 1)
  assert.equal(state.picker.value, '\u5206\u7ec4')
  assert.equal(dom.window.sessionStorage.getItem('new-api-group:4'), '\u5206\u7ec4')
  assert.equal(state.resets, 1)
  assert.equal(state.requests.at(-1).options.headers.get('X-NewAPI-Group'), encodeURIComponent('\u5206\u7ec4'))
})

test('empty authorization clears models and prevents stale or forged group selections', async (t) => {
  const { state, dom, window } = await setup(t)
  const oldSelect = state.picker.onChange
  state.groups = {}
  await state.intervals.find(({ delay }) => delay === 15000).fn()
  assert.equal(state.picker.items.length, 0)
  assert.equal(state.picker.value, '')
  assert.equal(state.picker.disabled, true)
  assert.equal(state.shownModels.length, 0)
  assert.equal(dom.window.sessionStorage.getItem('new-api-group:4'), '')
  await oldSelect('default')
  await oldSelect('unauthorized')
  assert.equal(state.picker.value, '')
  assert.equal(state.resets, 1)
  const response = await window.fetch('/api/models')
  assert.deepEqual(await response.json(), { data: [] })
})

test('configured image group is absent from chat choices and image models survive chat group changes', async (t) => {
  const { state, window } = await setup(t, {
    imageGroup: 'images', imageModels: [{ id: 'custom-image' }],
    groups: { default: {}, images: {}, chat2: {} },
  })
  assert.deepEqual(Array.from(state.picker.items, ({ value }) => value), ['default', 'chat2'])
  assert.equal(state.imageStudio.group, 'images')
  assert.equal(state.imageStudio.models[0].id, 'custom-image')
  await state.picker.onChange('chat2')
  assert.equal(state.imageStudio.models[0].id, 'custom-image')
  await window.fetch('/api/chat/completions')
  assert.equal(state.requests.at(-1).options.headers.get('X-NewAPI-Group'), 'chat2')
  await state.picker.onChange('images')
  assert.equal(state.picker.value, 'chat2')
})

test('saving image group refreshes chat exclusion and clears unavailable image models', async (t) => {
  const { state, dom } = await setup(t, {
    imageGroup: 'images', imageModels: [{ id: 'old-image' }],
    groups: { default: {}, images: {} },
  })
  state.imageGroup = 'default'
  state.imageModels = []
  state.imageError = 'Group unavailable'
  dom.window.dispatchEvent(new dom.window.Event('new-api-image-config-changed'))
  await flush()
  await flush()
  assert.deepEqual(Array.from(state.picker.items, ({ value }) => value), ['images'])
  assert.equal(state.picker.value, 'images')
  assert.equal(state.imageStudio.group, 'default')
  assert.equal(state.imageStudio.models.length, 0)
  assert.equal(state.imageStudio.error, 'Group unavailable')
})

test('image-only account has no chat models and late image responses cannot survive identity changes', async (t) => {
  const { state, window } = await setup(t, {
    imageGroup: 'default', groups: { default: {} },
  })
  assert.equal(state.picker.items.length, 0)
  assert.equal(state.picker.disabled, true)
  assert.deepEqual(await (await window.fetch('/api/models')).json(), { data: [] })
  let resolve
  state.pendingImages = new Promise((done) => { resolve = done })
  const refresh = state.intervals.find(({ delay }) => delay === 15000).fn()
  await flush()
  state.onMismatch()
  resolve({ group: 'default', models: [{ id: 'old-account-image' }] })
  await refresh
  assert.equal(state.imageStudio.models.length, 0)
})

test('native user store uses canonical name without merging different identities', async () => {
  let value = { email: 'alice@example.com', name: 'Old', bio: 'Keep me' }
  let subscriber
  let modelValues
  const user = {
    subscribe: (fn) => { subscriber = fn; fn(value) },
    update: (fn) => { value = fn(value); subscriber?.(value) },
  }
  const context = vm.createContext({})
  const dependency = new vm.SyntheticModule(['u', 'd'], function () {
    this.setExport('u', user)
    this.setExport('d', { set: (models) => { modelValues = models } })
  }, { context })
  const pickerDependency = new vm.SyntheticModule(['groupPicker'], function () {
    this.setExport('groupPicker', { set: () => {} })
  }, { context })
  const imageDependency = new vm.SyntheticModule(['imageStudio'], function () {
    this.setExport('imageStudio', { set: () => {} })
  }, { context })
  const module = new vm.SourceTextModule(readFileSync(join(__dirname, 'studio-native.js'), 'utf8'), { context })
  await module.link((specifier) => specifier.includes('studio-group-picker') ? pickerDependency :
    specifier.includes('studio-image-state') ? imageDependency : dependency)
  await module.evaluate()
  let mismatched = false
  module.namespace.watchUser({ email: 'alice@example.com', username: 'Alice' }, () => { mismatched = true })
  assert.equal(value.name, 'Alice')
  assert.equal(value.bio, 'Keep me')
  module.namespace.syncUser({ email: 'alice@example.com', username: 'Renamed' })
  assert.equal(value.name, 'Renamed')
  value = { email: 'bob@example.com', name: 'Bob' }
  subscriber(value)
  assert.equal(mismatched, true)
  assert.equal(value.name, 'Bob')
  module.namespace.setModels([{ id: 'per-group-model' }])
  assert.equal(modelValues[0].id, 'per-group-model')
})
