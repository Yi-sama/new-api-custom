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

const COMMON_IMAGE_SIZES = [
  { value: '1:1', size: '1024x1024' },
  { value: '3:2', size: '1536x1024' }, { value: '2:3', size: '1024x1536' },
  { value: '16:9', size: '1536x864' }, { value: '9:16', size: '864x1536' },
  { value: '512x512', size: '512x512' }, { value: '768x768', size: '768x768' },
  { value: '1024x768', size: '1024x768' }, { value: '768x1024', size: '768x1024' },
  { value: '1280x720', size: '1280x720' }, { value: '720x1280', size: '720x1280' },
  { value: '1920x1080', size: '1920x1080' }, { value: '1080x1920', size: '1080x1920' },
  { value: '4:3', size: '1152x864' }, { value: '3:4', size: '864x1152' },
  { value: '1536x1536', size: '1536x1536' }, { value: '2048x2048', size: '2048x2048' },
  { value: '2:1', size: '1536x768' }, { value: '1:2', size: '768x1536' },
  { value: '21:9', size: '2016x864' }, { value: '9:21', size: '864x2016' },
]

export function imageSizeOptions(model) {
  const offered = Array.isArray(model?.image_size_options) ? model.image_size_options : []
  if (model?.id === 'gpt-image-2') {
    // The pinned bridge rejects low-area squares and non-16-aligned 1080 edges.
    return COMMON_IMAGE_SIZES.filter((option) =>
      !['512x512', '768x768', '1920x1080', '1080x1920'].includes(option.size))
  }
  if (['gpt-image-1', 'gpt-image-1-mini', 'gpt-image-1.5'].includes(model?.id)) {
    return COMMON_IMAGE_SIZES.filter((option) => ['1:1', '3:2', '2:3'].includes(option.value))
  }
  if (['dall-e', 'dall-e-2'].includes(model?.id)) return [
    { value: '1:1', size: '1024x1024' }, { value: '512x512', size: '512x512' },
    { value: '256x256', size: '256x256' },
  ]
  if (model?.id === 'dall-e-3') return [
    { value: '1:1', size: '1024x1024' }, { value: '7:4', size: '1792x1024' },
    { value: '4:7', size: '1024x1792' },
  ]
  if (!offered.length) return COMMON_IMAGE_SIZES
  const seen = new Set()
  return offered.filter((option) => option && typeof option.value === 'string' &&
    typeof option.size === 'string' && option.value !== '5:4' &&
    !seen.has(option.size) && seen.add(option.size))
}

export function saveImagePreference(name, value) {
  const field = name === 'model' ? 'newApiImageModel' : name === 'size' ? 'newApiImageSize' : null
  if (!field) return Promise.reject(new Error('Unknown image preference'))
  const normalized = name === 'model' ? (typeof value === 'string' && value ? value : null) :
    (typeof value === 'string' && value ? value : 'auto')
  return savePreference(field, normalized)
}

export function saveChatModels(value) {
  if (!Array.isArray(value) || !value.length || value.some((model) => typeof model !== 'string' || !model)) {
    return Promise.resolve()
  }
  return savePreference('models', [...value])
}

function savePreference(field, normalized) {
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
