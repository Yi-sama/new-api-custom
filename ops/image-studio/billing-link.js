import { G as mount } from '/studio-v21/chunks/DtFjAPax.js'
import { C as ChevronLeft } from '/studio-v21/chunks/BECweHH6.js'

(() => {
  const billingEndpoint = '/api/image-studio/billing'
  const usageEndpoint = '/api/usage'
  const consoleHref = 'http://localhost:3000/dashboard'
  let consoleRow

  function isVisible(element) {
    const rect = element.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0
  }

  function hasClass(element, name) {
    return String(element.className || '')
      .split(/\s+/)
      .includes(name)
  }

  function formatQuota(value, data) {
    if (data.display_type === 'TOKENS') {
      return `${Math.round(value).toLocaleString()} tokens`
    }
    const amount = value / data.quota_per_unit
    if (data.display_type === 'CNY') {
      return `¥${(amount * data.usd_exchange_rate).toFixed(2)}`
    }
    return `$${amount.toFixed(2)}`
  }

  async function refreshBilling() {
    const balance = document.getElementById('new-api-billing-summary')
    if (!balance) return
    try {
      const response = await fetch(billingEndpoint, {
        credentials: 'same-origin',
        cache: 'no-store',
      })
      if (!response.ok) throw new Error('billing request failed')
      const data = await response.json()
      balance.textContent = `余额：${formatQuota(data.remaining_quota, data)} · 已用：${formatQuota(data.used_quota, data)}`
    } catch {
      balance.textContent = '余额：暂不可用'
    }
  }

  async function refreshOnlineCount() {
    const online = document.getElementById('new-api-online-count')
    if (!online) return
    try {
      const response = await fetch(usageEndpoint, {
        credentials: 'same-origin',
        cache: 'no-store',
      })
      if (!response.ok) throw new Error('usage request failed')
      const data = await response.json()
      if (!Number.isSafeInteger(data.user_count) || data.user_count < 0) {
        throw new Error('invalid online count')
      }
      online.textContent = `在线：${data.user_count.toLocaleString()}`
      online.dataset.state = 'ready'
    } catch {
      online.textContent = '在线：暂不可用'
      online.dataset.state = 'unavailable'
    }
  }

  function refresh() {
    void refreshBilling()
    void refreshOnlineCount()
  }

  function findActionInsertion(group, button) {
    // Native buttons live inside Tooltip wrappers, not directly in the group.
    let first = button
    while (first.parentElement !== group) first = first.parentElement
    return { group, first }
  }

  function findTopActionGroup() {
    const visibleNavs = [...document.querySelectorAll('nav')].filter((element) =>
      isVisible(element)
    )
    const topNav =
      visibleNavs.find((element) =>
        element.querySelector('#navbar-bg-gradient-to-b')
      ) ||
      visibleNavs.find(
        (element) => hasClass(element, 'sticky') && hasClass(element, 'top-0')
      )
    if (!topNav) return null

    const getVisibleButtons = (root) =>
      [...root.querySelectorAll('button, [role="button"]')]
        .filter((element) => element.id !== 'new-api-billing-badge')
        .filter((element) => isVisible(element))
        .filter(
          (element) =>
            !element.closest(
              '[role="dialog"], [aria-live], [data-sonner-toaster], [data-sonner-toast]'
            )
        )
        .sort(
          (first, second) =>
            first.getBoundingClientRect().left -
            second.getBoundingClientRect().left
        )

    const nativeGroup = [...topNav.querySelectorAll('div')].find((group) => {
      if (
        !hasClass(group, 'flex-none') ||
        !hasClass(group, 'flex') ||
        !hasClass(group, 'items-center') ||
        !hasClass(group, 'self-center')
      ) {
        return false
      }
      return getVisibleButtons(group).length >= 2
    })
    if (nativeGroup) {
      return findActionInsertion(nativeGroup, getVisibleButtons(nativeGroup)[0])
    }

    const candidates = getVisibleButtons(topNav)

    if (candidates.length < 2) return null
    const first = candidates[candidates.length - 2]
    const second = candidates[candidates.length - 1]
    let group = first.parentElement
    while (group && group !== topNav) {
      if (
        group.contains(second) &&
        hasClass(group, 'flex-none') &&
        hasClass(group, 'flex') &&
        hasClass(group, 'items-center') &&
        hasClass(group, 'self-center')
      ) {
        return findActionInsertion(group, first)
      }
      group = group.parentElement
    }
    return null
  }

  function createConsoleRow(newChat) {
    const row = document.createElement('div')
    row.id = 'new-api-console-link-row'
    row.className = newChat.parentElement.className

    const link = document.createElement('a')
    link.id = 'new-api-console-link'
    link.href = consoleHref
    link.className = newChat.className
    link.draggable = false
    link.setAttribute('aria-label', '返回 New API 控制台')

    const icon = document.createElement('div')
    icon.className = 'self-center flex size-4 shrink-0 items-center justify-center'
    icon.setAttribute('aria-hidden', 'true')
    mount(ChevronLeft, { target: icon, props: { className: 'size-4', strokeWidth: '1.5' } })

    const labelWrapper = document.createElement('div')
    labelWrapper.className = 'flex flex-1 self-center translate-y-[0.5px]'

    const label = document.createElement('div')
    label.className = 'self-center text-[0.8125rem] leading-5'
    label.textContent = '返回控制台'

    labelWrapper.append(label)
    link.append(icon, labelWrapper)
    row.append(link)
    return row
  }

  function createBalanceBadge() {
    const badge = document.createElement('span')
    badge.id = 'new-api-billing-badge'
    badge.setAttribute('aria-live', 'polite')
    const summary = document.createElement('span')
    summary.id = 'new-api-billing-summary'
    summary.textContent = '余额：读取中'
    const online = document.createElement('span')
    online.id = 'new-api-online-count'
    online.textContent = '在线：读取中'
    online.dataset.state = 'loading'
    online.title = 'Open WebUI 最近 3 分钟内活跃的用户数，每 30 秒刷新'
    badge.append(summary, online)
    return badge
  }

  function mountControls() {
    document.getElementById('new-api-billing-bar')?.remove()

    // The native sidebar also has a hidden shortcut button with the same ID.
    const newChat = [...document.querySelectorAll('a#sidebar-new-chat-button')].find(isVisible)
    const newChatRow = newChat?.parentElement
    if (newChatRow?.parentElement) {
      if (!consoleRow) consoleRow = createConsoleRow(newChat)
      if (consoleRow.nextElementSibling !== newChatRow) {
        newChatRow.before(consoleRow)
      }
    }

    const actionGroup = findTopActionGroup()
    let badge = document.getElementById('new-api-billing-badge')
    if (actionGroup) {
      const wasMissing = !badge
      if (wasMissing) badge = createBalanceBadge()
      if (
        badge.parentElement !== actionGroup.group ||
        badge.nextSibling !== actionGroup.first
      ) {
        badge.remove()
        actionGroup.group.insertBefore(badge, actionGroup.first)
      }
      if (wasMissing) void refresh()
    }
  }

  mountControls()
  const observer = new MutationObserver(mountControls)
  observer.observe(document.body, { childList: true, subtree: true })
  window.setInterval(mountControls, 2000)
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) void refresh()
  })
  window.setInterval(refresh, 30000)
})()
