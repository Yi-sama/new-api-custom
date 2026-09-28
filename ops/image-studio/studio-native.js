import { u as user, d as models } from '/studio-v21/chunks/BTPG6at6.js'
import { groupPicker } from '/static/studio-group-picker.js?v=21'
import { imageStudio } from '/static/studio-image-state.js?v=21'

let current

export function syncUser(context) {
  current = context
  user.update((value) => value && value.email === context.email && value.name !== context.username ?
    { ...value, name: context.username } : value)
}

export function watchUser(context, onMismatch, onReady = () => {}) {
  current = context
  let ready = false
  user.subscribe((value) => {
    if (!value) return
    if (value.email !== current.email) {
      onMismatch()
    } else if (value.name !== current.username) {
      syncUser(current)
    }
    if (value.email === current.email && !ready) {
      ready = true
      onReady()
    }
  })
}

export async function readImageStudio() {
  const response = await fetch('/api/v1/images/studio', {
    headers: { Authorization: `Bearer ${localStorage.token}` },
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (!response.ok) throw new Error('Image models unavailable')
  return response.json()
}

export function setImageStudio(value) {
  imageStudio.set(value)
}

export function setModels(value) {
  models.set(Array.isArray(value) ? value : [])
}

export function setGroupPicker(value) {
  groupPicker.set(value)
}
