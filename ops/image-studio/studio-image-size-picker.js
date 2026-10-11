import {
  p as push, b as pop, s as setupStores, e as storeGet, g as getContext,
  f as template, c as child, n as sibling, r as reset, a as append,
  t as effect, d as setText, x as preEffect, y as resetEffects,
} from '/studio-v21/chunks/DtFjAPax.js'
import { S as Select } from '/studio-v21/chunks/44B912oc.js'
import { C as ChevronDown } from '/studio-v21/chunks/m7hrjezZ.js'
import { C as Check } from '/studio-v21/chunks/BUVtCm5D.js'
import { imageStudio, imageSizeOptions } from '/static/studio-image-state.js?v=21'

const rowTemplate = template('<fieldset class="new-api-image-size-picker"><!> <span class="new-api-image-size-error" role="alert"> </span></fieldset>')
const triggerTemplate = template('<span class="new-api-image-size-label"> </span> <!>', 1)
const itemTemplate = template('<span class="new-api-image-size-label"> </span> <span><!></span>', 1)

export function ImageSizePicker(anchor, props) {
  push(props, false)
  const [stores, cleanup] = setupStores()
  const i18nStore = getContext('i18n')
  const i18n = () => storeGet(i18nStore, '$i18n', stores)
  const zh = () => i18n().language?.startsWith('zh')
  const state = () => storeGet(imageStudio, '$imageStudio', stores)
  const selectedModel = () => state().models.find((model) => model.id === props.model)
  const options = () => imageSizeOptions(selectedModel())
  const value = () => typeof props.value === 'string' && props.value ? props.value : 'auto'
  const selected = () => options().find((option) => option.size === value() || option.value === value())
  const valid = () => value() === 'auto' || Boolean(selected())
  const items = () => {
    const result = [{ value: 'auto', label: zh() ? '\u81ea\u52a8' : 'Auto' },
      ...options().map((option) => ({ value: option.size, label: option.size.replace('x', ':') }))]
    if (!valid()) {
      result.push({ value: value(), label: zh() ? '\u539f\u5c3a\u5bf8\u4e0d\u53ef\u7528' : 'Previous size unavailable' })
    }
    return result
  }
  const selectedValue = () => selected()?.size || value()
  const selectedLabel = () => items().find((item) => item.value === selectedValue())?.label || value()
  const row = rowTemplate()
  const error = row.querySelector('.new-api-image-size-error')
  const errorText = child(error, true)
  reset(error)

  preEffect(() => [props.value, state(), props.model], () => {
    if (state().loading || state().error || !selectedModel()) return
    const option = selected()
    if (option && value() !== option.size) {
      props.onChange(option.size)
    } else if (value().includes(':') && !imageSizeOptions().some((item) => item.value === value())
      && !option) {
      // Retired arbitrary ratios cannot remain hidden in a restored chat request.
      props.onChange('auto')
    }
  })
  resetEffects()

  Select(child(row), {
    get value() { return selectedValue() },
    get items() { return items() },
    side: 'top',
    align: 'start',
    maxHeight: '18rem',
    triggerClass: 'new-api-image-size-trigger flex items-center gap-1 text-[0.8125rem] font-normal text-gray-600 dark:text-gray-300',
    contentClass: 'new-api-image-size-options min-w-[10.625rem]',
    onChange: (next) => {
      if (next === 'auto' || options().some((option) => option.size === next)) props.onChange(next)
    },
    $$slots: {
      trigger: (target) => {
        const fragment = triggerTemplate()
        const label = fragment.firstChild
        const text = child(label, true)
        reset(label)
        ChevronDown(sibling(label, 2), { className: 'size-2.5 shrink-0', strokeWidth: '2.5' })
        effect(() => {
          setText(text, selectedLabel())
          label.title = selectedLabel()
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
          check.style.visibility = option.item.value === selectedValue() ? 'visible' : 'hidden'
        })
        append(target, fragment)
      },
    },
    $$legacy: true,
  })
  reset(row)
  effect(() => {
    const invalid = zh() ? '\u5f53\u524d\u6a21\u578b\u4e0d\u652f\u6301\u6b64\u5c3a\u5bf8' : 'Unsupported size for this model'
    row.dataset.invalid = String(!valid())
    setText(errorText, valid() ? '' : invalid)
    error.title = valid() ? '' : invalid
    const title = zh() ? '\u56fe\u50cf\u5c3a\u5bf8' : 'Image size'
    row.querySelector('.new-api-image-size-trigger')?.setAttribute('aria-label', `${title}: ${selectedLabel()}`)
  })
  append(anchor, row)
  pop()
  cleanup()
}
