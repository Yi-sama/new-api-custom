// Native component hook for the pinned Open WebUI v0.11.4 Svelte build.
import {
  p as push, b as pop, s as setupStores, e as storeGet, w as writable,
  g as getContext, f as template, c as child, n as sibling, r as reset,
  a as append, t as effect, d as setText,
} from '/studio-v21/chunks/DtFjAPax.js'
import { S as Select } from '/studio-v21/chunks/44B912oc.js'
import { C as ChevronDown } from '/studio-v21/chunks/m7hrjezZ.js'
import { C as Check } from '/studio-v21/chunks/BUVtCm5D.js'

export const groupPicker = writable({ value: '', items: [], disabled: true, error: false })

const rowTemplate = template('<fieldset id="new-api-group-picker"><span id="new-api-group-label"> </span> <div class="min-w-0 flex-1"><!></div></fieldset>')
const triggerTemplate = template('<span class="min-w-0 flex-1 truncate text-right"> </span> <!>', 1)
const itemTemplate = template('<span class="min-w-0 flex-1 truncate text-left"> </span> <span class="new-api-group-ratio"> </span> <span class="shrink-0"><!></span>', 1)

export function GroupPicker(anchor, props) {
  push(props, false)
  const [stores, cleanup] = setupStores()
  const state = () => storeGet(groupPicker, '$groupPicker', stores)
  const i18nStore = getContext('i18n')
  const t = (key) => storeGet(i18nStore, '$i18n', stores).t(key)
  const row = rowTemplate()
  const label = child(row)
  const labelText = child(label, true)
  reset(label)
  const control = sibling(label, 2)
  const selectAnchor = child(control)

  Select(selectAnchor, {
    get value() { return state().value },
    get items() { return state().items },
    get placeholder() { return t('Group') },
    align: 'end',
    triggerClass: 'new-api-group-trigger relative h-8 w-full flex items-center px-1.5 py-1.5 text-[0.8125rem] font-normal text-gray-700 hover:text-gray-900 dark:text-gray-200 dark:hover:text-gray-100',
    contentClass: 'min-w-[10.625rem] model-selector-child-menu new-api-group-options',
    onChange: (value) => {
      if (!state().disabled) void state().onChange(value)
    },
    $$slots: {
      trigger: (target, selected) => {
        const fragment = triggerTemplate()
        const text = fragment.firstChild
        const value = child(text, true)
        reset(text)
        const icon = sibling(text, 2)
        ChevronDown(icon, { className: 'size-2.5 shrink-0', strokeWidth: '2.5' })
        effect(() => {
          setText(value, selected.selectedLabel)
          text.title = selected.selectedLabel
        })
        append(target, fragment)
      },
      item: (target, option) => {
        const fragment = itemTemplate()
        const text = fragment.firstChild
        const value = child(text, true)
        reset(text)
        const ratio = sibling(text, 2)
        const ratioText = child(ratio, true)
        reset(ratio)
        const check = sibling(ratio, 2)
        Check(child(check), { className: 'size-3.5' })
        reset(check)
        effect(() => {
          setText(value, option.item.label)
          text.title = option.item.label
          setText(ratioText, option.item.ratio || '')
          check.style.visibility = option.selected ? 'visible' : 'hidden'
        })
        append(target, fragment)
      },
    },
    $$legacy: true,
  })
  reset(control)
  reset(row)
  effect(() => {
    row.disabled = state().disabled
    row.title = state().error ? t('Model list is temporarily unavailable') : ''
    setText(labelText, t('Group'))
    const button = control.querySelector('button')
    button?.setAttribute('aria-labelledby', 'new-api-group-label')
  })
  append(anchor, row)
  pop()
  cleanup()
}
