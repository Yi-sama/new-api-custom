import {
  p as push, b as pop, s as setupStores, e as storeGet, g as getContext,
  f as template, c as child, n as sibling, r as reset, a as append,
  t as effect, d as setText,
} from '/studio-v21/chunks/DtFjAPax.js'
import { S as Select } from '/studio-v21/chunks/44B912oc.js'
import { C as ChevronDown } from '/studio-v21/chunks/m7hrjezZ.js'
import { C as Check } from '/studio-v21/chunks/BUVtCm5D.js'
import { imageStudio } from '/static/studio-image-state.js?v=21'

const CUSTOM = '__new_api_custom_size__'
const CUSTOM_SIZE = /^(\d+)x(\d+)$/i
const rowTemplate = template('<fieldset class="new-api-image-size-picker"><!> <span class="new-api-image-size-editor" hidden><label><span class="sr-only"> </span><input type="number" min="1" step="1" inputmode="numeric"></label><span aria-hidden="true">x</span><label><span class="sr-only"> </span><input type="number" min="1" step="1" inputmode="numeric"></label><button type="button"><!></button></span> <span class="new-api-image-size-error" role="alert"> </span></fieldset>')
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
  const supported = () => {
    const seen = new Set()
    return (Array.isArray(selectedModel()?.image_size_options) ? selectedModel().image_size_options : [])
      .filter((option) => option && typeof option.value === 'string' && typeof option.size === 'string' && !seen.has(option.value) && seen.add(option.value))
  }
  const value = () => props.value || 'auto'
  const custom = () => CUSTOM_SIZE.test(value())
  const valid = () => value() === 'auto' || custom() || supported().some((option) => option.value === value())
  const optionLabel = (option) => `${option.value} \u00b7 ${option.size.replace('x', '\u00d7')}`
  const items = () => {
    const result = [{ value: 'auto', label: zh() ? '\u81ea\u52a8' : 'Auto' }, ...supported().map((option) => ({ value: option.value, label: optionLabel(option) }))]
    if (!valid()) result.push({ value: value(), label: `${value()} \u00b7 ${zh() ? '\u4e0d\u652f\u6301' : 'Unsupported'}` })
    result.push({ value: CUSTOM, label: custom() ? `${zh() ? '\u81ea\u5b9a\u4e49' : 'Custom'} \u00b7 ${value().replace('x', '\u00d7')}` : (zh() ? '\u81ea\u5b9a\u4e49' : 'Custom') })
    return result
  }
  const row = rowTemplate()
  const selectAnchor = child(row)
  const editor = row.querySelector('.new-api-image-size-editor')
  const editorLabels = editor.querySelectorAll('label')
  const widthTextContainer = editorLabels[0].querySelector('span')
  const widthText = child(widthTextContainer, true)
  reset(widthTextContainer)
  const width = editorLabels[0].querySelector('input')
  const heightTextContainer = editorLabels[1].querySelector('span')
  const heightText = child(heightTextContainer, true)
  reset(heightTextContainer)
  const height = editorLabels[1].querySelector('input')
  const confirm = editor.querySelector('button')
  const error = row.querySelector('.new-api-image-size-error')
  const errorText = child(error, true)
  reset(error)
  let editing = false

  const labels = () => ({
    title: zh() ? '\u56fe\u50cf\u5c3a\u5bf8' : 'Image size',
    width: zh() ? '\u5bbd\u5ea6' : 'Width',
    height: zh() ? '\u9ad8\u5ea6' : 'Height',
    confirm: zh() ? '\u786e\u8ba4\u81ea\u5b9a\u4e49\u5c3a\u5bf8' : 'Confirm custom size',
    invalid: zh() ? '\u5f53\u524d\u6a21\u578b\u4e0d\u652f\u6301\u6b64\u5c3a\u5bf8' : 'Unsupported size for this model',
  })
  const setEditing = (next) => {
    editing = next
    editor.hidden = !next
    row.dataset.editing = String(next)
    if (!next) return
    const match = value().match(CUSTOM_SIZE)
    const fallback = supported()[0]?.size.match(CUSTOM_SIZE)
    width.value = match?.[1] || fallback?.[1] || '1024'
    height.value = match?.[2] || fallback?.[2] || '1024'
    queueMicrotask(() => width.focus())
  }
  const commit = () => {
    if (!width.checkValidity() || !height.checkValidity() || !width.value || !height.value) {
      ;(width.checkValidity() && width.value ? height : width).reportValidity()
      return
    }
    props.onChange(`${Number(width.value)}x${Number(height.value)}`)
    setEditing(false)
  }
  const onKeydown = (event) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      setEditing(false)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      commit()
    }
  }

  Select(selectAnchor, {
    get value() { return custom() ? CUSTOM : value() },
    get items() { return items() },
    side: 'top',
    align: 'start',
    triggerClass: 'new-api-image-size-trigger flex items-center gap-1 text-[0.8125rem] font-normal text-gray-600 dark:text-gray-300',
    contentClass: 'new-api-image-size-options min-w-[10.625rem]',
    onChange: (next) => next === CUSTOM ? setEditing(true) : (setEditing(false), props.onChange(next)),
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
  Check(child(confirm), { className: 'size-3.5' })
  reset(confirm)
  reset(editor)
  reset(row)
  confirm.addEventListener('click', commit)
  width.addEventListener('keydown', onKeydown)
  height.addEventListener('keydown', onKeydown)
  effect(() => {
    const text = labels()
    setText(widthText, text.width)
    setText(heightText, text.height)
    confirm.title = text.confirm
    confirm.setAttribute('aria-label', text.confirm)
    row.dataset.invalid = String(!valid())
    setText(errorText, valid() ? '' : text.invalid)
    const button = row.querySelector('.new-api-image-size-trigger')
    button?.setAttribute('aria-label', `${text.title}: ${items().find((item) => item.value === (custom() ? CUSTOM : value()))?.label || value()}`)
  })
  append(anchor, row)
  pop()
  cleanup()
}
