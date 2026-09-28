import {
  p as push, b as pop, s as setupStores, e as storeGet, g as getContext,
  f as template, c as child, n as sibling, r as reset, a as append,
  t as effect, d as setText, x as preEffect, y as resetEffects,
} from '/studio-v21/chunks/DtFjAPax.js'
import { S as Select } from '/studio-v21/chunks/44B912oc.js'
import { C as ChevronDown } from '/studio-v21/chunks/m7hrjezZ.js'
import { C as Check } from '/studio-v21/chunks/BUVtCm5D.js'
import { imageStudio } from '/static/studio-image-state.js?v=21'
export { imageModels } from '/static/studio-image-state.js?v=21'

export function imageSelectionError(i18n) {
  return i18n.language?.startsWith('zh') ?
    '\u8bf7\u9009\u62e9\u5f53\u524d\u5206\u7ec4\u53ef\u7528\u7684\u751f\u56fe\u6a21\u578b\uff0c\u6216\u5173\u95ed\u56fe\u50cf\u751f\u6210' :
    'Select an available image model for this group, or turn off image generation'
}

const rowTemplate = template('<fieldset class="new-api-image-picker"><!></fieldset>')
const triggerTemplate = template('<span class="new-api-image-model-label"> </span> <!>', 1)
const itemTemplate = template('<span class="new-api-image-model-label"> </span> <span><!></span>', 1)

export function ImagePicker(anchor, props) {
  push(props, false)
  const [stores, cleanup] = setupStores()
  const i18nStore = getContext('i18n')
  const i18n = () => storeGet(i18nStore, '$i18n', stores)
  const state = () => storeGet(imageStudio, '$imageStudio', stores)
  const available = () => state().loading || state().error ? [] : state().models
  const items = () => available().map((model) => ({ value: model.id, label: model.name || model.id }))
  const placeholder = () => i18n().language?.startsWith('zh') ?
    (state().loading ? '\u52a0\u8f7d\u4e2d' : !state().group ? '\u672a\u914d\u7f6e\u751f\u56fe\u5206\u7ec4' :
      state().error ? '\u751f\u56fe\u6a21\u578b\u6682\u4e0d\u53ef\u7528' : available().length ? '\u9009\u62e9\u751f\u56fe\u6a21\u578b' : '\u65e0\u53ef\u7528\u751f\u56fe\u6a21\u578b') :
    (state().loading ? 'Loading' : !state().group ? 'Image group not configured' :
      state().error ? 'Image models unavailable' : available().length ? 'Select image model' : 'No image models')
  const row = rowTemplate()

  // Null means a fresh/legacy draft; an unavailable explicit choice never falls back.
  preEffect(() => [props.value, state()], () => {
    if (props.value == null && available().length) {
      props.onChange((available().find((model) => model.id === 'gpt-image-1') || available()[0]).id)
    }
  })
  resetEffects()
  Select(child(row), {
    get value() { return props.value || '' },
    get items() { return items() },
    get placeholder() { return placeholder() },
    side: 'top',
    align: 'start',
    triggerClass: 'new-api-image-trigger flex items-center gap-1 text-[0.8125rem] font-normal text-gray-600 dark:text-gray-300',
    contentClass: 'new-api-image-options min-w-[10.625rem]',
    onChange: (value) => {
      if (!row.disabled && available().some((model) => model.id === value)) (props.onSelect || props.onChange)(value)
    },
    $$slots: {
      trigger: (target, selected) => {
        const fragment = triggerTemplate()
        const label = fragment.firstChild
        const text = child(label, true)
        reset(label)
        ChevronDown(sibling(label, 2), { className: 'size-2.5 shrink-0', strokeWidth: '2.5' })
        effect(() => {
          setText(text, selected.selectedLabel)
          label.title = selected.selectedLabel
        })
        append(target, fragment)
      },
      item: (target, option) => {
        const fragment = itemTemplate()
        const label = fragment.firstChild
        const text = child(label, true)
        reset(label)
        const check = sibling(label, 2)
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
  reset(row)
  effect(() => {
    row.disabled = state().loading || !available().length
    row.dataset.invalid = String(!!props.value && !available().some((model) => model.id === props.value))
    const button = row.querySelector('button')
    button?.setAttribute('aria-label', `${i18n().t('Image Generation')}: ${items().find((item) => item.value === props.value)?.label || placeholder()}`)
  })
  append(anchor, row)
  pop()
  cleanup()
}
