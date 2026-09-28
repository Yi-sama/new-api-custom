import { w as writable, b5 as get } from '/studio-v21/chunks/DtFjAPax.js'
import { e as settings } from '/studio-v21/chunks/BTPG6at6.js'

export const imageStudio = writable({ group: '', models: [], loading: true, error: '' })
let preferenceSave = Promise.resolve()

export function imageModels() {
  const state = get(imageStudio)
  return state.loading || state.error ? [] : state.models
}

export function imagePreferences(value = get(settings)) {
  return {
    model: typeof value?.newApiImageModel === 'string' && value.newApiImageModel ? value.newApiImageModel : null,
    size: typeof value?.newApiImageSize === 'string' && value.newApiImageSize ? value.newApiImageSize : 'auto',
  }
}

export function saveImagePreference(name, value) {
  const field = name === 'model' ? 'newApiImageModel' : name === 'size' ? 'newApiImageSize' : null
  if (!field) return Promise.reject(new Error('Unknown image preference'))
  const normalized = name === 'model' ? (typeof value === 'string' && value ? value : null) :
    (typeof value === 'string' && value ? value : 'auto')
  const token = localStorage.token
  const save = preferenceSave.catch(() => {}).then(async () => {
    const response = await fetch('/api/v1/users/user/settings/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      credentials: 'same-origin',
      body: JSON.stringify({ ui: { [field]: normalized } }),
    })
    if (!response.ok) throw new Error('Failed to save image preferences')
    if (localStorage.token === token) settings.update((current) => ({ ...current, [field]: normalized }))
    return normalized
  })
  preferenceSave = save
  return save
}
