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
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSidebarData } from '@/hooks/use-sidebar-data'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { useAuthStore } from '@/stores/auth-store'

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {} }),
}))

describe('Tavern navigation', () => {
  beforeEach(() => useAuthStore.getState().auth.reset())

  it('requires login for the public Tavern tab', () => {
    const { result } = renderHook(() => useTopNavLinks())
    expect(result.current).toContainEqual({
      title: 'Tavern',
      href: '/tavern',
      requiresAuth: true,
    })
  })

  it('opens the same tab for logged-in users and respects the chat menu configuration', () => {
    useAuthStore.getState().auth.setUser({ id: 7, username: 'tester', role: 1 })
    const { result } = renderHook(() => ({
      top: useTopNavLinks(),
      sidebar: useSidebarData(),
    }))
    expect(result.current.top).toContainEqual({
      title: 'Tavern',
      href: '/tavern',
      requiresAuth: false,
    })
    expect(
      result.current.sidebar.navGroups.find((item) => item.id === 'chat')?.items
    ).toContainEqual(
      expect.objectContaining({
        title: 'Tavern',
        url: '/tavern',
        configUrls: ['/playground'],
      })
    )
  })
})
