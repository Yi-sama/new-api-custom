import {
  p as push, b as pop, s as setupStores, e as storeGet, w as writable,
  g as getContext, o as onMount, f as template, c as child, n as sibling,
  r as reset, a as append, t as effect, d as setText,
} from '/studio-v21/chunks/DtFjAPax.js'
import { S as Select } from '/studio-v21/chunks/44B912oc.js'
import { C as ChevronDown } from '/studio-v21/chunks/m7hrjezZ.js'
import { C as Check } from '/studio-v21/chunks/BUVtCm5D.js'
import { i as init } from '/studio-v21/chunks/CWPhcWBV.js'

const rowTemplate = template('<fieldset class="new-api-image-group-setting"><label> </label> <div><!></div> <span role="status"> </span></fieldset>')
const triggerTemplate = template('<span class="new-api-image-model-label"> </span> <!>', 1)
const itemTemplate = template('<span class="new-api-image-model-label"> </span> <span><!></span>', 1)

export function ImageGroupSetting(anchor, props) {
  push(props, false)
  const [stores, cleanup] = setupStores()
  const i18nStore = getContext('i18n')
  const i18n = () => storeGet(i18nStore, '$i18n', stores)
  const zh = () => i18n().language?.startsWith('zh')
  const options = writable({ loading: true, items: [], error: false })
  const state = () => storeGet(options, '$options', stores)
  onMount(() => {
    let active = true
    void fetch('/api/image-studio/groups', { credentials: 'same-origin', cache: 'no-store' })
      .then(async (response) => {
        if (!response.ok) throw new Error('Groups unavailable')
        const result = await response.json()
        if (!result.success) throw new Error('Groups unavailable')
        if (active) options.set({
          loading: false, error: false,
          items: Object.entries(result.data).map(([value, details]) => ({
            value, label: details.desc && details.desc !== value ? `${value} (${details.desc})` : value,
          })),
        })
      }).catch(() => { if (active) options.set({ loading: false, items: [], error: true }) })
    return () => { active = false }
  })
  init()
  const row = rowTemplate()
  const label = child(row)
  const labelText = child(label, true)
  reset(label)
  const container = sibling(label, 2)
  Select(child(container), {
    get value() { return props.value || '' },
    get items() {
      const items = [{ value: '', label: zh() ? '\u672a\u914d\u7f6e' : 'Not configured' }, ...state().items]
      if (props.value && !items.some((item) => item.value === props.value)) {
        items.push({ value: props.value, label: props.value })
      }
      return items
    },
    triggerClass: 'new-api-image-group-select flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300',
    contentClass: 'new-api-image-options min-w-[10.625rem]',
    onChange: (value) => { if (!row.disabled) props.onChange(value) },
    $$slots: {
      trigger: (target, selected) => {
        const fragment = triggerTemplate()
        const text = child(fragment.firstChild, true)
        reset(fragment.firstChild)
        ChevronDown(sibling(fragment.firstChild, 2), { className: 'size-3' })
        effect(() => setText(text, selected.selectedLabel))
        append(target, fragment)
      },
      item: (target, option) => {
        const fragment = itemTemplate()
        const text = child(fragment.firstChild, true)
        reset(fragment.firstChild)
        const check = sibling(fragment.firstChild, 2)
        Check(child(check), { className: 'size-3.5' })
        reset(check)
        effect(() => {
          setText(text, option.item.label)
          check.style.visibility = option.selected ? 'visible' : 'hidden'
        })
        append(target, fragment)
      },
    },
    $$legacy: true,
  })
  reset(container)
  const status = sibling(container, 2)
  const statusText = child(status, true)
  reset(status)
  reset(row)
  effect(() => {
    row.disabled = state().loading || state().error
    const title = zh() ? '\u751f\u56fe\u5206\u7ec4' : 'Image generation group'
    setText(labelText, title)
    container.querySelector('button')?.setAttribute('aria-label', title)
    setText(statusText, state().error ? (zh() ? '\u5206\u7ec4\u52a0\u8f7d\u5931\u8d25' : 'Could not load groups') : '')
  })
  append(anchor, row)
  pop()
  cleanup()
}
