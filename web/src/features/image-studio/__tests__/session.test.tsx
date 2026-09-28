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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ImageStudio } from '..'

const statusState = vi.hoisted(() => ({
  value: { image_studio_enabled: true } as Record<string, unknown>,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusState.value }),
}))

vi.mock('@/lib/api', () => ({
  api: { post: vi.fn() },
}))

const postSession = vi.mocked(api.post)

describe('Image Studio session handoff', () => {
  beforeEach(() => {
    statusState.value = { image_studio_enabled: true }
    postSession.mockReset()
  })

  it('redirects to the server-provided URL after session creation', async () => {
    postSession.mockResolvedValue({
      data: { success: true, data: { url: 'https://images.example.com/' } },
    })
    const redirect = vi.fn()

    render(<ImageStudio redirect={redirect} />)

    await waitFor(() =>
      expect(redirect).toHaveBeenCalledWith('https://images.example.com/')
    )
    expect(postSession).toHaveBeenCalledWith(
      '/api/image-studio/session',
      undefined,
      expect.objectContaining({ skipBusinessError: true })
    )
  })

  it('offers a retry after the session request fails', async () => {
    postSession
      .mockRejectedValueOnce(new Error('Unavailable'))
      .mockResolvedValueOnce({
        data: { success: true, data: { url: 'https://images.example.com/' } },
      })
    const redirect = vi.fn()

    render(<ImageStudio redirect={redirect} />)

    await screen.findByRole('alert')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))

    await waitFor(() =>
      expect(redirect).toHaveBeenCalledWith('https://images.example.com/')
    )
  })

  it('rejects a script URL without redirecting', async () => {
    postSession.mockResolvedValue({
      data: { success: true, data: { url: 'javascript:alert(1)' } },
    })
    const redirect = vi.fn()

    render(<ImageStudio redirect={redirect} />)

    await screen.findByRole('alert')
    expect(redirect).not.toHaveBeenCalled()
  })
})
