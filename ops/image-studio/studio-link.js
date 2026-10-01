(() => {
  const originalFetch = window.fetch.bind(window)
  const current = new URL(window.location.href)
  const newApiOrigin =
    current.hostname === 'localhost' || current.hostname.endsWith('.localhost')
      ? `${current.protocol}//${current.hostname}:3000`
      : current.hostname.startsWith('studio.')
        ? `${current.protocol}//api.${current.hostname.slice('studio.'.length)}`
        : current.origin
  const entryHref = `${newApiOrigin}/image-studio`
  let context
  let groups = {}
  let group = ''
  let native
  let changing = false
  let leaving = false
  let modelError = false
  let modelRequestId = 0
  let imageGroup = ''
  let imageRequestId = 0
  let userReady = false

  function returnToAccount() {
    if (leaving) return
    leaving = true
    imageRequestId++
    native?.setImageStudio({ group: '', models: [], loading: false, error: 'Session changed' })
    syncGroupPicker()
    localStorage.removeItem('token')
    sessionStorage.removeItem('selectedModels')
    window.location.replace(entryHref)
  }

  async function readContext() {
    const response = await originalFetch('/api/image-studio/context', {
      credentials: 'same-origin',
      cache: 'no-store',
    })
    if (response.status === 401 || response.status === 403) returnToAccount()
    if (!response.ok) throw new Error('New API session unavailable')
    const next = await response.json()
    if (context && next.email !== context.email) {
      returnToAccount()
      throw new Error('New API account changed')
    }
    context = next
    native?.syncUser(context)
    return next
  }

  async function readGroups() {
    const configResponse = await originalFetch('/api/v1/images/studio/config', {
      credentials: 'same-origin', cache: 'no-store',
    })
    if (!configResponse.ok) throw new Error('Image group configuration unavailable')
    const config = await configResponse.json()
    const imageGroupChanged = imageGroup !== config.group
    imageGroup = config.group || ''
    if (imageGroupChanged) {
      imageRequestId++
      native?.setImageStudio({ group: imageGroup, models: [], loading: true, error: '' })
    }
    const response = await originalFetch('/api/image-studio/groups', {
      credentials: 'same-origin',
      cache: 'no-store',
    })
    if (response.status === 401 || response.status === 403) returnToAccount()
    if (!response.ok) throw new Error('New API groups unavailable')
    const result = await response.json()
    if (!result.success) throw new Error('New API groups unavailable')
    groups = Object.fromEntries(Object.entries(result.data).filter(([value]) => value !== imageGroup))
    const saved = group || sessionStorage.getItem(`new-api-group:${context.id}`)
    const next = Object.hasOwn(groups, saved) ? saved :
      Object.hasOwn(groups, context.group) ? context.group : Object.keys(groups)[0] || ''
    const selectionChanged = group !== next
    group = next
    syncGroupPicker()
    if (selectionChanged && native) await reloadModels()
  }

  async function reloadImageModels() {
    if (!native || !userReady || leaving) return
    const requestId = ++imageRequestId
    try {
      const result = await native.readImageStudio()
      if (leaving || requestId !== imageRequestId) return
      if (result.group !== imageGroup) {
        await readGroups()
        if (result.group !== imageGroup) throw new Error('Image group configuration changed')
        if (leaving) return
      }
      native.setImageStudio({ group: imageGroup, models: result.models || [], loading: false, error: result.error || '' })
    } catch (error) {
      if (!leaving && requestId === imageRequestId) {
        native.setImageStudio({ group: imageGroup, models: [], loading: false, error: error.message })
      }
    }
  }

  const ready = (async () => {
    await readContext()
    await readGroups()
  })()

  // Install before native modules start so their first model request is scoped.
  window.fetch = async (input, options) => {
    const url = new URL(typeof input === 'string' || input instanceof URL ? input : input.url, location.href)
    const scoped = url.origin === location.origin &&
      (url.pathname.startsWith('/api/') || url.pathname.startsWith('/openai/'))
    if (!scoped || url.pathname.startsWith('/api/image-studio/')) {
      return originalFetch(input, options)
    }
    await ready
    if (leaving) throw new Error('New API session changed')
    if (!group && url.pathname === '/api/models') return Response.json({ data: [] })
    const headers = new Headers(options?.headers || (input instanceof Request ? input.headers : undefined))
    headers.set('X-NewAPI-Group', encodeURIComponent(group))
    headers.set('X-NewAPI-Identity', context.email)
    const response = await originalFetch(input, { ...options, headers })
    if (response.status === 401 && location.pathname !== '/auth') returnToAccount()
    return response
  }

  async function reloadModels() {
    const requestId = ++modelRequestId
    changing = true
    modelError = false
    syncGroupPicker()
    sessionStorage.setItem(`new-api-group:${context.id}`, group)
    sessionStorage.removeItem('selectedModels')
    native.setModels([])
    window.dispatchEvent(new Event('new-api-group-changed'))
    try {
      if (!group) return
      const response = await window.fetch('/api/models', { credentials: 'same-origin', cache: 'no-store' })
      if (!response.ok) throw new Error('New API models unavailable')
      const result = await response.json()
      if (!leaving && requestId === modelRequestId) native.setModels(result.data)
    } catch (error) {
      if (!leaving && requestId === modelRequestId) modelError = true
      console.error(error)
    } finally {
      if (requestId === modelRequestId) {
        changing = false
        syncGroupPicker()
      }
    }
  }

  function syncGroupPicker() {
    native?.setGroupPicker({
      value: group,
      items: Object.entries(groups).map(([value, details]) => {
        let ratio = Number.isFinite(details.ratio) ? `${details.ratio}\u00d7` :
          typeof details.ratio === 'string' ? details.ratio : ''
        if (Number.isFinite(details.ratio_min) && Number.isFinite(details.ratio_max)) {
          ratio = details.ratio_min === details.ratio_max ? `${details.ratio_min}\u00d7` :
            `${details.ratio_min}-${details.ratio_max}\u00d7`
        }
        return {
          value,
          label: details.desc && details.desc !== value ? `${value} (${details.desc})` : value,
          ratio,
        }
      }),
      disabled: changing || leaving || !group,
      error: modelError,
      onChange: async (value) => {
        if (changing || leaving || value === group || !Object.hasOwn(groups, value)) return
        group = value
        await reloadModels()
      },
    })
  }

  async function start() {
    try {
      await ready
      native = await import('/static/studio-native.js?v=21')
      native.watchUser(context, returnToAccount, () => {
        userReady = true
        void reloadImageModels()
      })
      syncGroupPicker()
      let checking = false
      const check = async () => {
        if (checking || changing || leaving) return
        checking = true
        try {
          await readContext()
          await readGroups()
          await reloadImageModels()
        } catch (error) {
          console.error(error)
        } finally {
          checking = false
        }
      }
      window.setInterval(check, 15000)
      document.addEventListener('visibilitychange', () => { if (!document.hidden) void check() })
      window.addEventListener('focus', check)
      window.addEventListener('new-api-image-config-changed', check)
    } catch (error) {
      console.error(error)
    }
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, { once: true })
  else void start()
})()
