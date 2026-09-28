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

import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { useAuthStore } from '@/stores/auth-store'

const statusState = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusState.value }),
}))

describe('Image Studio navigation', () => {
  beforeEach(() => {
    statusState.value = {}
    useAuthStore.getState().auth.reset()
  })

  it('keeps the entry visible when Open WebUI is not configured', () => {
    const { result } = renderHook(() => useTopNavLinks())

    expect(result.current).toContainEqual({
      title: 'Image Studio',
      href: '/image-studio',
      requiresAuth: true,
    })
  })

  it('keeps the same entry when Open WebUI is configured', () => {
    statusState.value = { image_studio_enabled: true }

    const { result } = renderHook(() => useTopNavLinks())

    expect(result.current).toContainEqual(
      expect.objectContaining({ href: '/image-studio' })
    )
  })
})
