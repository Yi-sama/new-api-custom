const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join, basename } = require('node:path')
const { test } = require('node:test')
const vm = require('node:vm')
const { execFileSync } = require('node:child_process')
const { JSDOM } = require(require.resolve('jsdom', { paths: [join(__dirname, '../../web')] }))

// Test against the pinned container's actual compiled Select, icons and Svelte runtime.
const chunks = process.env.STUDIO_WEBUI_CHUNKS || join(__dirname, '../../.local-tests/studio-native-v15/chunks')
const runtimePath = '/studio-v23/chunks/DtFjAPax.js'
const pickerPath = '/static/studio-group-picker.js?v=23'
const imagePickerPath = '/static/studio-image-picker.js?v=23'
const imageSizePath = '/static/studio-image-size-picker.js?v=23'
const imageSettingsPath = '/static/studio-image-settings.js?v=23'
const imageStatePath = '/static/studio-image-state.js?v=23'

async function setup(t, modelSelector = false, admin = false, image = false) {
  const dom = new JSDOM('<main><button id="model-trigger">Model</button><div id="model-popup"><div id="search-row"><input id="model-search-input"></div></div></main>', {
    url: 'http://localhost:3002/', pretendToBeVisual: true, runScripts: 'outside-only',
  })
  t.after(() => dom.window.close())
  const context = dom.getInternalVMContext()
  const requests = []
  dom.window.__newApiToastErrors = []
  dom.window.fetch = async (url, options) => {
    requests.push({ url, options })
    return { ok: true, json: async () => ({ success: true, data: { default: {}, images: { desc: 'Images' } } }) }
  }
  dom.window.queueMicrotask = queueMicrotask
  // JSDOM has no Web Animations API; finish native transitions without layout.
  dom.window.Element.prototype.animate = function (_frames, options) {
    const animation = { currentTime: options.duration, playState: 'finished', cancel() { this.onfinish = null } }
    queueMicrotask(() => animation.onfinish?.())
    return animation
  }
  const cache = new Map()
  function load(url) {
    if (cache.has(url)) return cache.get(url)
    const path = url.startsWith('/static/') ? join(__dirname, basename(url.split('?')[0])) : join(chunks, basename(url))
    let source = url.endsWith('/6m72j43W.js') ? execFileSync('python', ['-B', '-c',
      'import sys; from pathlib import Path; sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_model_selector; print(patch_model_selector(Path(sys.argv[2]).read_text(encoding="utf-8")))',
      __dirname, path], { encoding: 'utf8', timeout: 5000 }) : readFileSync(path, 'utf8')
    if (url.startsWith('/static/')) {
      source = source.replaceAll('/studio-v21/', '/studio-v23/').replaceAll('?v=21', '?v=23')
    }
    if (image === 'toggle' && url === imagePickerPath) {
      const patched = execFileSync('python', ['-B', '-c',
        'import sys; from pathlib import Path; sys.stdout.reconfigure(encoding="utf-8"); sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_chat; print(patch_chat(Path(sys.argv[2]).read_text(encoding="utf-8")))',
        __dirname, join(chunks, 'weExeAJk.js')], { encoding: 'utf8', maxBuffer: 2 * 1024 * 1024 })
      const start = patched.indexOf('var co=w=>')
      const end = patched.indexOf(';se(Ki,', start)
      source += `
import {p as Xt,b as Gt,m as I,l as r,k as c,u as xe,z as h,f as O,c as v,n as E,r as f,t as V,a as b,q as le,W as NewApiBranch,h as fragment,i as first} from '${runtimePath}';
import {i as se} from '/studio-v23/chunks/BKqLVPYF.js';
import {s as Tt} from '/studio-v23/chunks/BLgWsK8X.js';
import {T as Dt} from '/studio-v23/chunks/DYjgh7dg.js';
import {ImageSizePicker} from '${imageSizePath}';
import {saveImagePreference as NewApiSaveImagePreference} from '${imageStatePath}';
export function ImageToggleHarness(anchor, props) {
 Xt(props,false);
 const tt=()=>props.enabled, n=()=>({t:key=>key}), mo=fn=>fn;
 const _t={error:value=>window.__newApiToastErrors.push(value)};
 const newApiImageModel=value=>value===undefined?props.value:props.onChange(value);
 const newApiImageSize=value=>value===undefined?props.size:props.onSizeChange(value);
 const NewApiImagePicker=ImagePicker;
 const NewApiImageSizePicker=ImageSizePicker;
 const fd=O('<button type="button"><!> <div class="hidden group-hover:block"><!></div></button>');
 const kg=()=>{},pn=()=>{};
 const outer=fragment(), Ki=first(outer);
 ${patched.slice(start, end)};
 se(Ki,render=>{if(tt())render(co)});
 b(anchor,outer);Gt();
}
`
    }
    const module = new vm.SourceTextModule(source, {
      context, identifier: url,
      initializeImportMeta: (meta) => { meta.url = `http://localhost:3002${url}` },
    })
    cache.set(url, module)
    for (const specifier of module.dependencySpecifiers) load(new URL(specifier, `http://localhost:3002${url}`).pathname + (specifier.includes('?') ? `?${specifier.split('?')[1]}` : ''))
    return module
  }
  const entry = load(image === 'settings' ? imageSettingsPath : image === 'size' ? imageSizePath : image ? imagePickerPath : modelSelector ? '/studio-v23/chunks/6m72j43W.js' : pickerPath)
  await entry.link((specifier, referencing) => load(new URL(specifier, `http://localhost:3002${referencing.identifier}`).pathname + (specifier.includes('?') ? `?${specifier.split('?')[1]}` : '')))
  await entry.evaluate()
  if (admin) cache.get('/studio-v23/chunks/BTPG6at6.js').namespace.u.set({ role: 'admin' })
  const runtime = cache.get(runtimePath).namespace
  const { GroupPicker, groupPicker } = cache.get(pickerPath)?.namespace || {}
  const { G: mount, J: unmount, H: flush, w: writable } = runtime
  const imageValue = runtime.m(null)
  const imageSize = runtime.m('auto')
  const enabled = runtime.m(true)
  const setImage = (value) => runtime.k(imageValue, value)
  const setSize = (value) => runtime.k(imageSize, value)
  const imageProps = {
    get value() { return runtime.l(imageValue) }, get enabled() { return runtime.l(enabled) },
    get size() { return runtime.l(imageSize) }, onChange: setImage, onSizeChange: setSize,
  }
  const sizeProps = { get value() { return runtime.l(imageSize) }, get model() { return runtime.l(imageValue) }, onChange: setSize }
  const imageState = cache.get(imageStatePath)?.namespace
  const imageStudio = imageState?.imageStudio
  const translated = { Group: '\u5206\u7ec4' }
  const i18n = writable({ t: (key) => translated[key] || key, language: 'zh-CN' })
  const state = {
    value: 'default',
    items: [{ value: 'default', label: 'Default', ratio: '1\u00d7' }, { value: 'vip', label: 'VIP', ratio: '0.5-1.5\u00d7' }],
    disabled: false,
    onChange: (value) => { state.value = value; groupPicker.set({ ...state }) },
  }
  groupPicker?.set(state)
  const target = dom.window.document.getElementById('model-popup')
  const anchor = dom.window.document.getElementById('search-row')
  const open = () => {
    const instance = mount(image === 'settings' ? entry.namespace.ImageGroupSetting : image === 'toggle' ? entry.namespace.ImageToggleHarness : image === 'size' ? entry.namespace.ImageSizePicker : image ? entry.namespace.ImagePicker : modelSelector ? entry.namespace.S : GroupPicker, {
      target, anchor: modelSelector ? undefined : anchor,
      props: image === 'size' ? sizeProps : image ? imageProps : modelSelector ? { id: 'test', selectionOnly: true, items: [] } : {},
      context: new Map([['i18n', i18n]]),
    })
    flush()
    return instance
  }
  const tick = async () => {
    flush()
    await new Promise(setImmediate)
    flush()
  }
  return { dom, open, unmount, tick, state, groupPicker, imageStudio, imageState, settings: cache.get('/studio-v23/chunks/BTPG6at6.js')?.namespace.e, setImage, setSize, imageProps, sizeProps, entry, requests, setEnabled: value => runtime.k(enabled, value) }
}

test('real WebUI Select mounts inside the model popup and cleans up its portaled options', async (t) => {
  const { dom, open, unmount, tick, state } = await setup(t)
  const document = dom.window.document
  assert.equal(document.getElementById('new-api-group-picker'), null)
  const instance = open()
  const picker = document.getElementById('new-api-group-picker')
  assert.equal(picker.parentElement.id, 'model-popup')
  assert.equal(picker.nextElementSibling.id, 'search-row')
  assert.equal(document.querySelector('select'), null)
  const trigger = picker.querySelector('button')
  assert.equal(trigger.textContent.trim(), 'Default')
  assert.ok(trigger.querySelector('svg'))
  trigger.click()
  await tick()
  const options = document.querySelector('.new-api-group-options')
  assert.ok(options.classList.contains('app-dropdown-menu'))
  assert.ok(options.classList.contains('model-selector-child-menu'))
  assert.equal(options.querySelectorAll('button').length, 2)
  assert.deepEqual([...options.querySelectorAll('.new-api-group-ratio')].map((el) => el.textContent), ['1\u00d7', '0.5-1.5\u00d7'])
  options.querySelectorAll('button')[1].click()
  await tick()
  assert.equal(state.value, 'vip')
  assert.equal(trigger.textContent.trim(), 'VIP')
  assert.equal(state.value, 'vip')
  trigger.click()
  await tick()
  await unmount(instance)
  assert.equal(document.querySelector('.new-api-group-options'), null)
  assert.equal(document.getElementById('new-api-group-picker'), null)
  const reopened = open()
  assert.equal(document.querySelector('.new-api-group-trigger').textContent.trim(), 'VIP')
  assert.equal(document.querySelectorAll('#new-api-group-picker').length, 1)
  await unmount(reopened)
})

test('patched real model selector owns the group picker and removes both menus on close', async (t) => {
  const { dom, open, unmount, tick, state } = await setup(t, true)
  const document = dom.window.document
  document.getElementById('search-row').remove()
  const instance = open()
  assert.equal(document.getElementById('new-api-group-picker'), null)
  const modelTrigger = document.getElementById('model-selector-test-button')
  modelTrigger.click()
  await tick()
  const picker = document.getElementById('new-api-group-picker')
  assert.ok(picker)
  assert.equal(picker.nextElementSibling.querySelector('input').id, 'model-search-input')
  assert.equal(modelTrigger.parentElement.contains(picker), false)
  picker.querySelector('button').click()
  await tick()
  assert.ok(document.querySelector('.new-api-group-options'), document.body.innerHTML)
  document.querySelectorAll('.new-api-group-options button')[1].click()
  await tick()
  assert.equal(state.value, 'vip')
  assert.equal(modelTrigger.getAttribute('aria-expanded'), 'true')
  assert.equal(document.querySelectorAll('#new-api-group-picker').length, 1)
  picker.querySelector('button').click()
  await tick()
  modelTrigger.click()
  await tick()
  assert.equal(document.getElementById('new-api-group-picker'), null)
  assert.equal(document.querySelector('.new-api-group-options'), null)
  modelTrigger.click()
  await tick()
  assert.equal(document.querySelectorAll('#new-api-group-picker').length, 1)
  assert.equal(document.querySelector('.new-api-group-trigger').textContent.trim(), 'VIP')
  document.querySelector('.new-api-group-trigger').click()
  await tick()
  document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  await tick()
  assert.equal(document.getElementById('new-api-group-picker'), null)
  assert.equal(document.querySelector('.new-api-group-options'), null)
  await unmount(instance)
})

test('real picker follows loading, empty permissions and revoked groups without recreating the popup', async (t) => {
  const { dom, open, unmount, tick, state, groupPicker } = await setup(t)
  const instance = open()
  const document = dom.window.document
  const picker = document.getElementById('new-api-group-picker')
  groupPicker.set({ ...state, disabled: true })
  await tick()
  assert.equal(picker.disabled, true)
  picker.querySelector('button').click()
  await tick()
  assert.equal(document.querySelector('.new-api-group-options'), null)
  groupPicker.set({ ...state, value: 'vip', items: [state.items[1]], disabled: false })
  await tick()
  assert.equal(picker.querySelector('button').textContent.trim(), 'VIP')
  groupPicker.set({ ...state, value: '', items: [], disabled: true })
  await tick()
  assert.equal(picker.disabled, true)
  assert.equal(picker.querySelector('button').textContent.trim(), '\u5206\u7ec4')
  await unmount(instance)
})

test('empty admin model popup keeps the group picker without connection setup prompts', async (t) => {
  const { dom, open, unmount, tick } = await setup(t, true, true)
  const document = dom.window.document
  document.getElementById('search-row').remove()
  const instance = open()
  document.getElementById('model-selector-test-button').click()
  await tick()
  assert.ok(document.getElementById('new-api-group-picker'))
  const manage = [...document.querySelectorAll('button')].find((button) => button.textContent === 'Manage Connections')
  assert.ok(manage)
  assert.equal(manage.hidden, true)
  assert.equal(manage.style.display, 'none')
  const prompt = manage.previousElementSibling
  assert.equal(prompt.hidden, true)
  assert.equal(prompt.style.display, 'none')
  assert.equal(prompt.previousElementSibling.textContent, 'No models available')
  await unmount(instance)
})

test('image picker lists every model in the independent image group without name or endpoint filtering', async (t) => {
  const fixture = await setup(t, false, false, true)
  const { dom, imageStudio, open, tick, unmount, imageProps } = fixture
  imageStudio.set({ group: 'images', loading: false, error: '', models: [
    { id: 'gpt-image-1' },
    { id: 'custom-model' },
  ] })
  const instance = open()
  await tick()
  const trigger = dom.window.document.querySelector('.new-api-image-trigger')
  assert.equal(imageProps.value, 'gpt-image-1')
  assert.equal(fixture.requests.length, 0, 'automatic default selection is draft-only')
  assert.equal(trigger.textContent.trim(), 'gpt-image-1')
  trigger.click()
  await tick()
  const buttons = dom.window.document.querySelectorAll('.new-api-image-options button')
  assert.deepEqual([...buttons].map((button) => button.textContent.trim()), ['gpt-image-1', 'custom-model'])
  buttons[1].click()
  await tick()
  assert.equal(imageProps.value, 'custom-model')
  assert.equal(trigger.textContent.trim(), 'custom-model')
  trigger.click()
  await tick()
  await unmount(instance)
  assert.equal(dom.window.document.querySelector('.new-api-image-options'), null)
})

test('revoked image selections do not silently fall back and new drafts get a default', async (t) => {
  const { dom, imageStudio, open, tick, unmount, imageProps, setImage } = await setup(t, false, false, true)
  const state = { group: 'images', loading: false, error: '' }
  imageStudio.set({ ...state, models: [{ id: 'old' }] })
  const instance = open()
  await tick()
  assert.equal(imageProps.value, 'old')
  imageStudio.set({ ...state, models: [], loading: true })
  await tick()
  assert.equal(dom.window.document.querySelector('fieldset.new-api-image-picker').disabled, true)
  imageStudio.set({ ...state, models: [{ id: 'other' }] })
  await tick()
  assert.equal(imageProps.value, 'old')
  assert.equal(dom.window.document.querySelector('.new-api-image-picker').dataset.invalid, 'true')
  assert.equal(dom.window.document.querySelector('.new-api-image-trigger').textContent.trim(), '\u9009\u62e9\u751f\u56fe\u6a21\u578b')
  setImage(null)
  await tick()
  assert.equal(imageProps.value, 'other')
  await unmount(instance)
})

test('image size picker lists pixel dimensions for production aliases without custom inputs', async (t) => {
  const { dom, imageStudio, imageState, settings, requests, open, tick, unmount, setImage, setSize, sizeProps } = await setup(t, false, false, 'size')
  settings.set({ newApiImageRatios: ['5:4'] })
  imageStudio.set({ group: 'images', loading: false, error: '', models: [
    { id: 'gpt-image-2.5-web', image_size_options: [] },
    { id: 'gpt-image-2.5-sunburst', image_size_options: [] },
    { id: 'gpt-image-2.5-flare', image_size_options: [] },
    { id: 'gpt-image-2', image_size_options: [{ value: '1:1', size: '1024x1024' }] },
  ] })
  setImage('gpt-image-2.5-web')
  const instance = open()
  await tick()
  const document = dom.window.document
  const trigger = document.querySelector('.new-api-image-size-trigger')
  assert.equal(sizeProps.value, 'auto')
  assert.equal(trigger.getAttribute('aria-label'), '\u56fe\u50cf\u5c3a\u5bf8: \u81ea\u52a8')
  const modelIds = ['gpt-image-2.5-web', 'gpt-image-2.5-sunburst', 'gpt-image-2.5-flare', 'gpt-image-2']
  const labels = [
    '1024:1024', '1536:1024', '1024:1536', '1536:864', '864:1536', '512:512', '768:768',
    '1024:768', '768:1024', '1280:720', '720:1280', '1920:1080', '1080:1920',
    '1152:864', '864:1152', '1536:1536', '2048:2048', '1536:768', '768:1536', '2016:864', '864:2016',
  ]
  for (const model of modelIds) {
    setImage(model)
    trigger.click()
    await tick()
    const buttons = [...document.querySelectorAll('.new-api-image-size-options button')]
    const available = model === 'gpt-image-2' ?
      labels.filter((label) => !['512:512', '768:768', '1920:1080', '1080:1920'].includes(label)) : labels
    assert.deepEqual(buttons.map((button) => button.textContent.trim()), ['\u81ea\u52a8', ...available], model)
    buttons.find((button) => button.textContent.trim() === '1536:864').click()
    await tick()
    assert.equal(sizeProps.value, '1536x864', 'presets send exact pixels accepted by the bridge, not an unknown ratio')
    assert.equal(trigger.textContent.trim(), '1536:864')
    assert.equal(document.querySelector('.new-api-image-size-picker').dataset.invalid, 'false')
  }
  assert.equal(document.querySelector('.new-api-image-size-editor'), null)
  assert.equal(document.querySelectorAll('#model-popup input[type=number]').length, 0)
  assert.equal(requests.length, 0, 'picker never writes arbitrary ratio lists')
  setSize('5:4')
  await tick()
  assert.equal(sizeProps.value, 'auto', 'retired ratio in a restored chat cannot be sent')
  assert.equal(trigger.textContent.trim(), '\u81ea\u52a8')
  setSize('4:3')
  await tick()
  assert.equal(sizeProps.value, '1152x864', 'old standard ratio is migrated to concrete pixels')
  assert.equal(trigger.textContent.trim(), '1152:864')
  const cases = modelIds.map(model => ({ model, options: imageState.imageSizeOptions({ id: model }) }))
  const checked = execFileSync('python', ['-B', '-c',
    'import json, sys; sys.path.insert(0, sys.argv[1]); from open_webui_bridge import bridge_image_generation_size as resolve\n' +
    'cases = json.load(sys.stdin)\n' +
    'for case in cases:\n' +
    ' for option in case["options"]:\n' +
    '  assert resolve(case["model"], {"image_generation_size": option["size"]}) == option["size"]\n' +
    '  if ":" in option["value"]:\n' +
    '   w, h = map(int, option["size"].split("x")); a, b = map(int, option["value"].split(":")); assert w * b == h * a\n' +
    'print(sum(len(case["options"]) for case in cases))',
    __dirname], { encoding: 'utf8', input: JSON.stringify(cases), timeout: 5000 })
  assert.equal(checked.trim(), '80', 'every displayed pixel preset roundtrips through the real backend policy')
  await unmount(instance)
  assert.equal(document.querySelector('.new-api-image-size-options'), null)
  assert.equal(document.querySelector('.new-api-image-size-picker'), null)
})

test('fixed-size models preserve actual supported sizes and unsupported choices do not silently change', async (t) => {
  const { dom, imageStudio, open, tick, unmount, setImage, setSize, sizeProps } = await setup(t, false, false, 'size')
  imageStudio.set({ group: 'images', loading: false, error: '', models: [
    { id: 'gpt-image-1', image_size_options: [] },
    { id: 'dall-e-3', image_size_options: [] },
  ] })
  setImage('gpt-image-1')
  const instance = open()
  await tick()
  const document = dom.window.document
  const trigger = document.querySelector('.new-api-image-size-trigger')
  trigger.click()
  await tick()
  let buttons = [...document.querySelectorAll('.new-api-image-size-options button')]
  assert.deepEqual(buttons.map((button) => button.textContent.trim()), ['\u81ea\u52a8', '1024:1024', '1536:1024', '1024:1536'])
  buttons[2].click()
  await tick()
  assert.equal(sizeProps.value, '1536x1024')
  setImage('dall-e-3')
  await tick()
  assert.equal(sizeProps.value, '1536x1024', 'model switch cannot transform a user dimension silently')
  assert.equal(document.querySelector('.new-api-image-size-picker').dataset.invalid, 'true')
  trigger.click()
  await tick()
  buttons = [...document.querySelectorAll('.new-api-image-size-options button')]
  buttons.find((button) => button.textContent.trim() === '1792:1024').click()
  await tick()
  assert.equal(sizeProps.value, '1792x1024')
  setSize('7:4')
  await tick()
  assert.equal(sizeProps.value, '1792x1024')
  await unmount(instance)
})

test('actual compiled image toggle removes picker and open menu on every disable', async (t) => {
  const { dom, open, unmount, tick, setEnabled, imageStudio } = await setup(t, false, false, 'toggle')
  imageStudio.set({ group: 'images', loading: false, error: '', models: [
    { id: 'custom-image', image_size_options: [{ value: '1:1', size: '1024x1024' }] },
  ] })
  const instance = open()
  for (const enabled of [true, false, true, false, true]) {
    setEnabled(enabled)
    await tick()
    assert.equal(dom.window.document.querySelectorAll('.new-api-image-picker').length, enabled ? 1 : 0)
    assert.equal(dom.window.document.querySelectorAll('.new-api-image-size-picker').length, enabled ? 1 : 0)
    if (enabled) {
      dom.window.document.querySelector('.new-api-image-size-trigger').click()
      await tick()
      assert.equal(dom.window.document.querySelectorAll('.new-api-image-size-options').length, 1)
    } else {
      assert.equal(dom.window.document.querySelector('.new-api-image-options'), null)
      assert.equal(dom.window.document.querySelector('.new-api-image-size-options'), null)
    }
  }
  await unmount(instance)
  assert.equal(dom.window.document.querySelectorAll('.new-api-image-picker').length, 0)
  assert.equal(dom.window.document.querySelectorAll('.new-api-image-size-picker').length, 0)
  assert.equal(dom.window.document.querySelector('.new-api-image-options'), null)
  assert.equal(dom.window.document.querySelector('.new-api-image-size-options'), null)
})

test('actual compiled picker callback reports a rejected preference save', async (t) => {
  const { dom, open, unmount, tick, imageStudio, imageProps } = await setup(t, false, false, 'toggle')
  dom.window.localStorage.token = 'toast-user-token'
  dom.window.fetch = async () => ({ ok: false })
  imageStudio.set({ group: 'images', loading: false, error: '', models: [
    { id: 'first-image' }, { id: 'second-image' },
  ] })
  const instance = open()
  await tick()
  assert.equal(imageProps.value, 'first-image')
  dom.window.document.querySelector('.new-api-image-trigger').click()
  await tick()
  dom.window.document.querySelectorAll('.new-api-image-options button')[1].click()
  await tick()
  await new Promise(setImmediate)
  assert.equal(imageProps.value, 'second-image')
  assert.deepEqual(dom.window.__newApiToastErrors, ['Failed to save image preferences'])
  await unmount(instance)
})

test('admin image group setting changes only the draft and retains native select behavior', async (t) => {
  const { dom, open, unmount, tick, imageProps, requests } = await setup(t, false, false, 'settings')
  const instance = open()
  await tick()
  const trigger = dom.window.document.querySelector('.new-api-image-group-select')
  assert.equal(trigger.getAttribute('aria-label'), '\u751f\u56fe\u5206\u7ec4')
  trigger.click()
  await tick()
  const options = dom.window.document.querySelectorAll('.new-api-image-options button')
  assert.deepEqual([...options].map((button) => button.textContent.trim()), ['\u672a\u914d\u7f6e', 'default', 'images (Images)'])
  options[2].click()
  await tick()
  assert.equal(imageProps.value, 'images')
  assert.equal(requests.length, 1)
  assert.equal(requests[0].url, '/api/image-studio/groups')
  assert.equal(requests[0].options.method, undefined)
  trigger.click()
  await tick()
  await unmount(instance)
  assert.equal(dom.window.document.querySelector('.new-api-image-options'), null)
})

test('image preferences persist per user while explicit drafts retain priority', async (t) => {
  const first = await setup(t, false, false, true)
  const plain = (value) => JSON.parse(JSON.stringify(value))
  first.dom.window.localStorage.token = 'user-a-token'
  first.settings.set({ newApiImageModel: 'server-model', newApiImageSize: '4:3' })
  assert.deepEqual(plain(first.imageState.imagePreferences()), { model: 'server-model', size: '4:3' })
  await first.imageState.saveImagePreference('model', 'saved-model')
  await first.imageState.saveImagePreference('size', '1280x720')
  assert.deepEqual(plain(first.imageState.imagePreferences()), { model: 'saved-model', size: '1280x720' })
  const savedModel = first.requests.at(-2)
  const savedSize = first.requests.at(-1)
  assert.equal(savedModel.url, '/api/v1/users/user/settings/update')
  assert.equal(savedModel.options.headers.Authorization, 'Bearer user-a-token')
  assert.deepEqual(JSON.parse(savedModel.options.body), { ui: { newApiImageModel: 'saved-model' } })
  assert.deepEqual(JSON.parse(savedSize.options.body), { ui: { newApiImageSize: '1280x720' } })
  const reloaded = await setup(t, false, false, true)
  reloaded.settings.set({ ...JSON.parse(savedModel.options.body).ui, ...JSON.parse(savedSize.options.body).ui })
  assert.deepEqual(plain(reloaded.imageState.imagePreferences()), { model: 'saved-model', size: '1280x720' }, 'a new module session hydrates from durable server settings')
  first.settings.set({ newApiImageModel: 'user-b-model', newApiImageSize: '1:1' })
  assert.deepEqual(plain(first.imageState.imagePreferences()), { model: 'user-b-model', size: '1:1' }, 'account settings replace prior user preferences without module leakage')

  const source = execFileSync('python', ['-B', '-c',
    'import sys; from pathlib import Path; sys.stdout.reconfigure(encoding="utf-8"); sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_chat; print(patch_chat(Path(sys.argv[2]).read_text(encoding="utf-8")))',
    __dirname, join(chunks, 'weExeAJk.js')], { encoding: 'utf8', maxBuffer: 2 * 1024 * 1024 })
  new vm.SourceTextModule(source)
  const start = source.indexOf('Bo=()=>') + 3
  const end = source.indexOf(',ta=()=>', start)
  const context = vm.createContext({
    o: () => ({ features: { enable_image_generation: true } }),
    u: () => ({ role: 'user', permissions: { features: { image_generation: true } } }),
    W: () => false, n: () => ({}), r: (value) => value,
    newApiImageSize: '4:3', newApiImageModel: 'custom-image', Kt: true, Wt: false, Qt: false,
    NewApiImageModels: () => [{ id: 'custom-image' }],
  })
  const requestFeatures = vm.runInContext(`(${source.slice(start, end)})`, context)
  assert.equal(requestFeatures().image_generation_size, '4:3')
  assert.equal(requestFeatures().image_generation_model, 'custom-image')
  assert.equal(requestFeatures().image_generation, true)
  context.Kt = false
  assert.equal(requestFeatures().image_generation, false)
  context.Kt = true
  context.NewApiImageModels = () => []
  assert.equal(requestFeatures().image_generation, false, 'ordinary chat remains available without an image group')
  assert.ok(source.includes('.catch(()=>_t.error(n().t("Failed to save image preferences")))'), 'persistence failures use the native localized error toast')
  assert.ok(source.includes('newApiImagePreference=NewApiImagePreferences(n()),newApiImageSize=I(newApiImagePreference.size),newApiImageModel=I(newApiImagePreference.model),Kt=I(!0)'))
  assert.equal(source.split('newApiImagePreference=NewApiImagePreferences(n())').length - 1, 4)
  const resetStart = source.indexOf('c(Kt,!0),newApiImagePreference=NewApiImagePreferences(n())')
  const resetEnd = source.indexOf(')', source.indexOf('c(newApiImageModel,newApiImagePreference.model)', resetStart)) + 1
  const freshSignals = { Kt: { value: false }, newApiImageSize: { value: 'auto' }, newApiImageModel: { value: null } }
  vm.runInContext(`(()=>{${source.slice(resetStart, resetEnd)}})()`, vm.createContext({
    ...freshSignals,
    n: () => ({ newApiImageModel: 'retired-model', newApiImageSize: '4:3' }),
    NewApiImagePreferences: (settings) => ({ model: settings.newApiImageModel, size: settings.newApiImageSize }),
    c: (signal, value) => { signal.value = value },
  }))
  assert.equal(freshSignals.newApiImageModel.value, 'retired-model', 'fresh chats restore the explicit preference without substituting an available model')
  assert.equal(freshSignals.newApiImageSize.value, '4:3')
  const draftStart = source.indexOf('Ji=async w=>') + 3
  const draftEnd = source.indexOf(',hi=w=>', draftStart)
  const signals = Object.fromEntries(['Nr', 'qe', 'Sr', 'Ht', 'hr', 'fr', 'Ot', 'Kt', 'Wt', 'newApiImageSize', 'newApiImageModel']
    .map((name) => [name, { value: null }]))
  const draftContext = vm.createContext({
    ...signals, _: () => false,
    c: (signal, value) => { signal.value = value },
    r: (signal) => signal.value,
  })
  const restore = vm.runInContext(`(${source.slice(draftStart, draftEnd)})`, draftContext)
  assert.equal(await restore(JSON.stringify({ prompt: 'draft' })), true)
  assert.equal(signals.Kt.value, true)
  signals.newApiImageSize.value = '4:3'
  signals.newApiImageModel.value = 'server-model'
  assert.equal(await restore(JSON.stringify({ prompt: 'legacy draft' })), true)
  assert.equal(signals.newApiImageSize.value, '4:3')
  assert.equal(signals.newApiImageModel.value, 'server-model')
  assert.equal(await restore(JSON.stringify({ imageGenerationEnabled: false, imageGenerationModel: 'custom-image', imageGenerationSize: '1280x720' })), true)
  assert.equal(signals.Kt.value, false)
  assert.equal(signals.newApiImageModel.value, 'custom-image')
  assert.equal(signals.newApiImageSize.value, '1280x720')

  const layout = readFileSync(join(chunks, '../2.mPOUt5NL.js'), 'utf8')
  const hydrate = layout.indexOf('const Z=await dp(localStorage.token);Z!=null&&Z.ui&&gr.set(Z.ui)')
  const ready = layout.indexOf('n(le,!0)', hydrate)
  const render = layout.indexOf('e(le)?q(V):q(U,-1)', ready)
  assert.ok(hydrate >= 0 && hydrate < ready && ready < render, 'native user settings hydrate before the authenticated Chat branch renders')
})

test('image preference failures do not claim persistence and writes stay ordered', async (t) => {
  const failed = await setup(t, false, false, true)
  failed.dom.window.localStorage.token = 'failed-user-token'
  failed.settings.set({ newApiImageModel: 'old-model', newApiImageSize: 'auto' })
  failed.dom.window.fetch = async () => ({ ok: false })
  await assert.rejects(failed.imageState.saveImagePreference('model', 'rejected-model'), /Failed to save image preferences/)
  assert.equal(failed.imageState.imagePreferences().model, 'old-model', 'rejected writes do not update the native settings store')

  const ordered = await setup(t, false, false, true)
  ordered.dom.window.localStorage.token = 'ordered-user-token'
  ordered.settings.set({ newApiImageModel: 'old-model', newApiImageSize: 'auto' })
  const pending = []
  ordered.dom.window.fetch = (url, options) => new Promise((resolve) => pending.push({ url, options, resolve }))
  const modelSave = ordered.imageState.saveImagePreference('model', 'new-model')
  const sizeSave = ordered.imageState.saveImagePreference('size', '4:3')
  await new Promise(setImmediate)
  assert.equal(pending.length, 1, 'the second write waits for the first')
  pending[0].resolve({ ok: true })
  await modelSave
  await new Promise(setImmediate)
  assert.equal(pending.length, 2)
  pending[1].resolve({ ok: true })
  await sizeSave
  assert.deepEqual(JSON.parse(pending[0].options.body), { ui: { newApiImageModel: 'new-model' } })
  assert.deepEqual(JSON.parse(pending[1].options.body), { ui: { newApiImageSize: '4:3' } })
  assert.equal(ordered.imageState.imagePreferences().model, 'new-model')
  assert.equal(ordered.imageState.imagePreferences().size, '4:3')

  const switched = await setup(t, false, false, true)
  switched.dom.window.localStorage.token = 'account-a-token'
  switched.settings.set({ newApiImageModel: 'account-a-model', newApiImageSize: 'auto' })
  let finish
  switched.dom.window.fetch = () => new Promise((resolve) => { finish = resolve })
  const inFlight = switched.imageState.saveImagePreference('model', 'account-a-new-model')
  await new Promise(setImmediate)
  switched.dom.window.localStorage.token = 'account-b-token'
  switched.settings.set({ newApiImageModel: 'account-b-model', newApiImageSize: '1:1' })
  finish({ ok: true })
  await inFlight
  assert.equal(switched.imageState.imagePreferences().model, 'account-b-model', 'an old account response cannot update the new account settings store')
  assert.equal(switched.imageState.imagePreferences().size, '1:1')
})

test('chat model selection saves account defaults without saving empty selections', async (t) => {
  const fixture = await setup(t, false, false, true)
  fixture.dom.window.localStorage.token = 'chat-user-token'
  fixture.settings.set({ newApiImageModel: 'image-model', newApiImageSize: '4:3' })
  await fixture.imageState.saveChatModels(['chat-model'])
  const request = fixture.requests.at(-1)
  assert.deepEqual(JSON.parse(request.options.body), { ui: { models: ['chat-model'] } })
  assert.equal(fixture.imageState.imagePreferences().model, 'image-model')
  const reloaded = await setup(t, false, false, true)
  reloaded.settings.set(JSON.parse(request.options.body).ui)
  let saved
  reloaded.settings.subscribe((value) => { saved = value })()
  assert.deepEqual(Array.from(saved.models), ['chat-model'])
  for (const value of [[], [''], ['chat-model', '']]) await fixture.imageState.saveChatModels(value)
  assert.equal(fixture.requests.length, 1)

  const source = execFileSync('python', ['-B', '-c',
    'import sys; from pathlib import Path; sys.stdout.reconfigure(encoding="utf-8"); sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_chat; print(patch_chat(Path(sys.argv[2]).read_text(encoding="utf-8")))',
    __dirname, join(chunks, 'weExeAJk.js')], { encoding: 'utf8', maxBuffer: 2 * 1024 * 1024 })
  const start = source.indexOf('set values(P){g(P);NewApiSaveChatModels')
  assert.ok(start >= 0)
  const end = source.indexOf('},$$legacy', start)
  let selected
  const errors = []
  const callback = vm.runInNewContext(`({${source.slice(start, end + 1)}})`, {
    g: (value) => { selected = value },
    NewApiSaveChatModels: fixture.imageState.saveChatModels,
    _t: { error: (value) => errors.push(value) }, s: () => ({ t: (value) => value }),
  })
  callback.values = ['other-model']
  await fixture.tick()
  assert.deepEqual(Array.from(selected), ['other-model'])
  assert.deepEqual(JSON.parse(fixture.requests.at(-1).options.body), { ui: { models: ['other-model'] } })
  fixture.dom.window.fetch = async () => ({ ok: false })
  callback.values = ['rejected-model']
  await fixture.tick()
  assert.deepEqual(errors, ['Failed to save model preferences'])
})

test('actual admin save persists the image group without requiring a shared key and notifies only on success', async () => {
  const source = execFileSync('python', ['-B', '-c',
    'import sys; from pathlib import Path; sys.stdout.reconfigure(encoding="utf-8"); sys.path.insert(0, sys.argv[1]); from patch_frontend import patch_image_settings; print(patch_image_settings(Path(sys.argv[2]).read_text(encoding="utf-8")))',
    __dirname, join(chunks, '../2.mPOUt5NL.js')], { encoding: 'utf8', maxBuffer: 4 * 1024 * 1024 })
  new vm.SourceTextModule(source)
  const component = source.slice(source.indexOf('function VT('))
  const start = component.indexOf('Ie=async()=>') + 3
  const end = component.indexOf(',pe=Ae=>', start)
  const saved = []
  const events = []
  let fail = false
  const config = {
    NEW_API_BRIDGE_ENABLED: true, NEW_API_IMAGE_GROUP: 'images',
    IMAGE_GENERATION_ENGINE: 'openai', IMAGES_OPENAI_API_KEY: '',
    ENABLE_IMAGE_GENERATION: true, IMAGE_GENERATION_MODEL: 'legacy-global',
    AUTOMATIC1111_PARAMS: '', IMAGES_OPENAI_API_PARAMS: '{}',
  }
  const context = vm.createContext({
    v: config, e: (value) => value,
    at: { error: () => {} }, t: () => ({ t: (value) => value }),
    He: (_object, value) => value, localStorage: { token: 'fixture-only' },
    Wb: async (_token, body) => { saved.push(body); return fail ? null : body },
    ur: { set: () => {} }, zl: async () => ({}), Ce: () => {},
    Event, window: { dispatchEvent: (event) => { events.push(event.type) } },
  })
  const save = vm.runInContext(`(${component.slice(start, end)})`, context)
  assert.ok(await save())
  assert.equal(saved[0].NEW_API_IMAGE_GROUP, 'images')
  assert.equal(saved[0].IMAGE_GENERATION_MODEL, 'legacy-global')
  assert.deepEqual(events, ['new-api-image-config-changed'])
  fail = true
  assert.equal(await save(), null)
  assert.equal(events.length, 1)
  config.NEW_API_BRIDGE_ENABLED = false
  assert.equal(await save(), null)
  assert.equal(saved.length, 2, 'non-bridge native key validation remains active')
})
