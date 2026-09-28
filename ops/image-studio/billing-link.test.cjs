const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join, basename } = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const { JSDOM } = require(
  require.resolve('jsdom', { paths: [join(__dirname, '../../web')] })
)

const script = readFileSync(join(__dirname, 'billing-link.js'), 'utf8')
const chunks = process.env.STUDIO_WEBUI_CHUNKS || join(__dirname, '../../.local-tests/studio-native-v15/chunks')
const markup = `
  <div id="user-menu-items" hidden>
    <div class="user-menu-row flex items-center w-full">
      <a href="/workspace">Workspace</a>
    </div>
  </div>
  <button id="sidebar-new-chat-button" hidden></button>
  <aside id="sidebar"><div id="sidebar-items">
    <div class="px-1 flex justify-center text-gray-700 dark:text-gray-300"><a id="sidebar-new-chat-button" class="group grow flex items-center space-x-2 rounded-xl px-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-900 transition outline-none" href="/" draggable="false"><div class="self-center flex size-4 shrink-0 items-center justify-center"></div><div class="flex flex-1 self-center translate-y-[0.5px]"><div class="self-center text-[0.8125rem] leading-5">新对话</div></div></a></div>
    <div data-id="workspace"><a id="sidebar-workspace-button" href="/workspace">工作空间</a></div>
    <!-- Svelte navigation boundary -->
    <div><a href="/notes">笔记</a></div>
  </div></aside>
  <nav class="sticky top-0">
    <div id="navbar-bg-gradient-to-b"></div>
    <div class="lg:mr-1 flex flex-none items-center gap-2 self-center" id="actions">
      <!-- Tooltip components wrap their native buttons in div.flex. -->
      <div class="flex" id="first-action"><button>Temporary Chat</button></div>
      <div class="flex" id="second-action"><button>Controls</button></div>
    </div>
  </nav>
  <div data-sonner-toaster aria-live="polite"><button>Login successful</button></div>
`

const billing = {
  remaining_quota: 1_000_000,
  used_quota: 250_000,
  display_type: 'USD',
  quota_per_unit: 500_000,
}
const flush = () => new Promise(setImmediate)

async function loadControls(
  t,
  { html = markup, ok = true, usage = { user_count: 1 }, usageOk = true } = {}
) {
  const dom = new JSDOM(`<!doctype html><body>${html}</body>`, {
    url: 'http://localhost:3002/',
    runScripts: 'outside-only',
  })
  const observers = []
  const NativeObserver = dom.window.MutationObserver
  dom.window.MutationObserver = class extends NativeObserver {
    constructor(callback) {
      super(callback)
      observers.push(this)
    }
  }
  t.after(() => {
    for (const observer of observers) observer.disconnect()
    dom.window.close()
  })
  // jsdom implements DOM mutations, but does not calculate layout.
  dom.window.HTMLElement.prototype.getBoundingClientRect = function () {
    return { width: this.closest('[hidden]') ? 0 : 100, height: 32, left: 0, top: 0 }
  }
  const requests = []
  const intervals = []
  dom.window.setInterval = (callback, delay) => intervals.push({ callback, delay })
  dom.window.fetch = async (path, options) => {
    requests.push({ path, options })
    if (path === '/api/usage') {
      return { ok: usageOk, json: async () => usage }
    }
    assert.equal(path, '/api/image-studio/billing')
    return { ok, json: async () => billing }
  }
  const context = dom.getInternalVMContext()
  const modules = new Map()
  const loadModule = (url) => {
    if (!modules.has(url)) {
      const source = url === '/static/billing-link.js' ? script : readFileSync(join(chunks, basename(url)), 'utf8')
      modules.set(url, new vm.SourceTextModule(source, { context, identifier: url }))
    }
    return modules.get(url)
  }
  const entry = loadModule('/static/billing-link.js')
  await entry.link((specifier, referencing) => loadModule(new URL(specifier, `http://localhost:3002${referencing.identifier}`).pathname))
  await entry.evaluate()
  await flush()
  return { document: dom.window.document, requests, intervals }
}

function assertPlacement(document) {
  const newChat = [...document.querySelectorAll('a#sidebar-new-chat-button')]
    .find((element) => !element.hidden)
  const newChatRow = newChat.parentElement
  const consoleRow = document.getElementById('new-api-console-link-row')
  const consoleLink = document.getElementById('new-api-console-link')
  const badge = document.getElementById('new-api-billing-badge')
  const actions = document.getElementById('actions')
  assert.equal(newChatRow.previousElementSibling, consoleRow)
  assert.equal(consoleRow.parentElement.id, 'sidebar-items')
  assert.equal(consoleRow.className, newChatRow.className)
  assert.equal(consoleLink.className, newChat.className)
  assert.ok(consoleLink.querySelector('svg[aria-hidden="true"]'))
  assert.equal(consoleLink.children[0].className, newChat.children[0].className)
  assert.equal(consoleLink.children[1].className, newChat.children[1].className)
  assert.equal(consoleLink.children[1].firstElementChild.className, newChat.children[1].firstElementChild.className)
  assert.equal(
    document.getElementById('new-api-console-link').href,
    'http://localhost:3000/dashboard'
  )
  assert.equal(badge.parentElement, actions)
  assert.equal(badge.nextElementSibling.id, 'first-action')
  assert.equal(document.querySelectorAll('#new-api-console-link-row').length, 1)
  assert.equal(document.querySelectorAll('#new-api-billing-badge').length, 1)
  assert.equal(document.querySelectorAll('#new-api-online-count').length, 1)
  assert.equal(
    document.getElementById('new-api-billing-summary').nextElementSibling,
    document.getElementById('new-api-online-count')
  )
  assert.equal(document.querySelector('[data-sonner-toaster]').children.length, 1)
}

test('mounts above New Chat and before the native tooltip-wrapped actions', async (t) => {
  const { document, requests } = await loadControls(t)
  assertPlacement(document)
  assert.equal(
    document.getElementById('new-api-billing-summary').textContent,
    '余额：$2.00 · 已用：$0.50'
  )
  assert.equal(requests[0].path, '/api/image-studio/billing')
  assert.equal(requests[0].options.credentials, 'same-origin')
  assert.equal(document.getElementById('new-api-online-count').textContent, '在线：1')
  const usageRequest = requests.find(({ path }) => path === '/api/usage')
  assert.equal(usageRequest.options.credentials, 'same-origin')
  assert.equal(usageRequest.options.cache, 'no-store')
  assert.equal(usageRequest.options.headers, undefined)
})

test('ordinary users without Workspace get the same console entry above New Chat', async (t) => {
  const { document } = await loadControls(t, {
    html: markup.replace('<div data-id="workspace"><a id="sidebar-workspace-button" href="/workspace">工作空间</a></div>', ''),
  })
  assertPlacement(document)
  const consoleRow = document.getElementById('new-api-console-link-row')
  assert.equal(consoleRow.nextElementSibling.querySelector('a').id, 'sidebar-new-chat-button')
  assert.equal(document.querySelector('#user-menu-items #new-api-console-link-row'), null)
})

test('billing failure keeps both controls and displays an unavailable balance', async (t) => {
  const { document } = await loadControls(t, { ok: false })
  assertPlacement(document)
  assert.equal(
    document.getElementById('new-api-billing-summary').textContent,
    '余额：暂不可用'
  )
  assert.equal(document.getElementById('new-api-online-count').textContent, '在线：1')
})

test('mounts after hydration and restores controls after navigation without duplicates', async (t) => {
  const { document } = await loadControls(t, { html: '' })
  document.body.innerHTML = markup
  await flush()
  assertPlacement(document)

  document.body.innerHTML = markup
  await flush()
  assertPlacement(document)

  document.body.append(document.createComment('unrelated page update'))
  await flush()
  assertPlacement(document)
})

test('refreshes the native active count every 30 seconds, including zero', async (t) => {
  const usage = { user_count: 4 }
  const { document, intervals } = await loadControls(t, { usage })
  const online = document.getElementById('new-api-online-count')
  assert.equal(online.textContent, '在线：4')
  assert.equal(online.dataset.state, 'ready')

  usage.user_count = 0
  intervals.find(({ delay }) => delay === 30000).callback()
  await flush()
  assert.equal(online.textContent, '在线：0')
  assert.equal(online.dataset.state, 'ready')
  assertPlacement(document)
})

test('usage access failure leaves the real balance visible without inventing an online count', async (t) => {
  const { document } = await loadControls(t, { usageOk: false })
  assert.equal(
    document.getElementById('new-api-billing-summary').textContent,
    '余额：$2.00 · 已用：$0.50'
  )
  const online = document.getElementById('new-api-online-count')
  assert.equal(online.textContent, '在线：暂不可用')
  assert.equal(online.dataset.state, 'unavailable')
})

test('invalid or missing counts are unavailable and recover on the next refresh', async (t) => {
  const usage = { user_count: 2 }
  const { document, intervals } = await loadControls(t, { usage })
  const online = document.getElementById('new-api-online-count')
  const refresh = intervals.find(({ delay }) => delay === 30000).callback
  for (const count of [undefined, null, -1, 1.5, '4']) {
    usage.user_count = count
    refresh()
    await flush()
    assert.equal(online.textContent, '在线：暂不可用')
    assert.equal(online.dataset.state, 'unavailable')
  }
  usage.user_count = 3
  refresh()
  await flush()
  assert.equal(online.textContent, '在线：3')
  assert.equal(online.dataset.state, 'ready')
})

test('opening and recreating the mobile sidebar restores the entry without duplicates', async (t) => {
  const hiddenOnly = '<button id="sidebar-new-chat-button" hidden></button>'
  const { document } = await loadControls(t, { html: hiddenOnly })
  assert.equal(document.getElementById('new-api-console-link-row'), null)
  document.body.innerHTML = markup
  await flush()
  assertPlacement(document)
  document.body.innerHTML = hiddenOnly
  await flush()
  assert.equal(document.getElementById('new-api-console-link-row'), null)
  document.body.innerHTML = markup
  await flush()
  assertPlacement(document)
})
