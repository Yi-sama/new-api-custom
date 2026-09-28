/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useNotificationStore } from '@/stores/notification-store'

import { GeoIPNotice } from '../geoip-notice'

let client: QueryClient

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 8, 21, 0, 30))
  localStorage.clear()
  useNotificationStore.setState(useNotificationStore.getInitialState())
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('loc=CN\n')))
})

afterEach(() => {
  vi.restoreAllMocks()
  client.clear()
  useNotificationStore.setState(useNotificationStore.getInitialState())
  localStorage.clear()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function renderNotice() {
  return render(
    <QueryClientProvider client={client}>
      <GeoIPNotice />
    </QueryClientProvider>
  )
}

describe('GeoIP daily dismissal', () => {
  it.each(['I understand', 'Close'])(
    'persists today when checked and closed with %s, surviving rehydration',
    async (button) => {
      const user = userEvent.setup()
      const view = renderNotice()
      const checkbox = await screen.findByRole('checkbox', {
        name: "Don't remind me today",
      })
      expect(checkbox).not.toBeChecked()
      await user.click(checkbox)
      expect(checkbox).toBeChecked()
      expect(screen.getByRole('dialog')).toBeVisible()
      await user.click(screen.getByRole('button', { name: button }))
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

      const saved = localStorage.getItem('notification-storage')
      if (saved === null) throw new Error('Dismissal preference was not saved')
      view.unmount()
      useNotificationStore.setState(useNotificationStore.getInitialState())
      localStorage.setItem('notification-storage', saved)
      await useNotificationStore.persist.rehydrate()
      renderNotice()

      expect(screen.queryByRole('dialog')).toBeNull()
      expect(useNotificationStore.getState().geoIPClosedUntilDate).toBe(
        'Mon Sep 21 2026'
      )
    }
  )

  it('shows again after an unchecked dismissal and a fresh page mount', async () => {
    const user = userEvent.setup()
    const view = renderNotice()
    await user.click(
      await screen.findByRole('button', { name: 'I understand' })
    )
    view.unmount()
    renderNotice()
    expect(await screen.findByRole('dialog')).toBeVisible()
  })

  it('shows again on the next local calendar date', async () => {
    const user = userEvent.setup()
    const view = renderNotice()
    await user.click(
      await screen.findByRole('checkbox', { name: "Don't remind me today" })
    )
    await user.click(screen.getByRole('button', { name: 'I understand' }))
    view.unmount()
    vi.setSystemTime(new Date(2026, 8, 22, 0, 1))
    renderNotice()
    expect(await screen.findByRole('dialog')).toBeVisible()
  })

  it('does not suppress GeoIP when a normal announcement was dismissed today', async () => {
    useNotificationStore
      .getState()
      .setClosedUntilDate(new Date().toDateString())
    renderNotice()
    expect(await screen.findByRole('dialog')).toBeVisible()
  })

  it('lets the keyboard select today and dismiss with Escape', async () => {
    const user = userEvent.setup()
    renderNotice()
    const checkbox = await screen.findByRole('checkbox', {
      name: "Don't remind me today",
    })
    checkbox.focus()
    await user.keyboard('[Space]')
    expect(checkbox).toBeChecked()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(useNotificationStore.getState().geoIPClosedUntilDate).toBe(
      'Mon Sep 21 2026'
    )
  })

  it('still closes when browser storage refuses the preference write', async () => {
    const user = userEvent.setup()
    renderNotice()
    await user.click(
      await screen.findByRole('checkbox', { name: "Don't remind me today" })
    )
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('Storage unavailable', 'SecurityError')
    })
    await user.click(screen.getByRole('button', { name: 'I understand' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
