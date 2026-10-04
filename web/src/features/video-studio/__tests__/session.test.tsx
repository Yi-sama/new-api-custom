import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { VideoStudio } from '..'
import type { VideoStudioSession } from '../api'

vi.mock('@/lib/api', () => ({
  api: { post: vi.fn(), get: vi.fn() },
}))

const postSession = vi.mocked(api.post)
const aliceSession: VideoStudioSession = {
  user_id: 1,
  username: 'alice',
  token: 'alice-token',
  expires_at: 1_800_000_000,
}
const bobSession: VideoStudioSession = {
  user_id: 2,
  username: 'bob',
  token: 'bob-token',
  expires_at: 1_800_000_100,
}
const renewedAliceSession: VideoStudioSession = {
  ...aliceSession,
  token: 'alice-renewed-token',
  expires_at: 1_800_000_100,
}

function setUser(id: number, username: string) {
  useAuthStore.getState().auth.setUser({ id, username, role: 1 })
}

function sendMessage(source: Window, origin: string, data: unknown) {
  window.dispatchEvent(new MessageEvent('message', { source, origin, data }))
}

function getFrameWindow(frame: HTMLIFrameElement): Window {
  const frameWindow = frame.contentWindow
  expect(frameWindow).not.toBeNull()
  if (!frameWindow) throw new Error('Iframe window is unavailable')
  return frameWindow
}

function renderStudio() {
  const router = createRouter({
    routeTree: createRootRoute({ component: VideoStudio }),
    history: createMemoryHistory(),
  })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

describe('Video Studio session handoff', () => {
  beforeEach(() => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
    postSession.mockReset()
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: { id: 1, username: 'alice', quota: 500000, used_quota: 0 },
      },
    })
    setUser(1, 'alice')
  })

  it('shows loading until the same-origin canvas reports ready', async () => {
    postSession.mockResolvedValue({
      data: { success: true, data: aliceSession },
    })

    renderStudio()

    expect(
      await screen.findByText('Loading Video Studio...')
    ).toBeInTheDocument()
    const frame = await screen.findByTitle('Video Studio')
    expect(frame).toHaveAttribute('src', '/video-canvas/video?v=2')

    sendMessage(
      getFrameWindow(frame as HTMLIFrameElement),
      window.location.origin,
      { type: 'new-api-video-ready' }
    )

    await waitFor(() =>
      expect(screen.queryByText('Loading Video Studio...')).toBeNull()
    )
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows an accessible error and retries session creation', async () => {
    postSession
      .mockRejectedValueOnce(new Error('disabled'))
      .mockResolvedValueOnce({
        data: { success: true, data: aliceSession },
      })

    renderStudio()

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Unable to load Video Studio'
    )
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByTitle('Video Studio')).toHaveAttribute(
      'src',
      '/video-canvas/video?v=2'
    )
    expect(postSession).toHaveBeenCalledTimes(2)
  })

  it('posts the in-memory session on load and valid requests only', async () => {
    postSession.mockResolvedValue({
      data: { success: true, data: aliceSession },
    })
    renderStudio()
    const frame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    const frameWindow = getFrameWindow(frame)
    const postMessage = vi.spyOn(frameWindow, 'postMessage')

    fireEvent.load(frame)
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'new-api-video-session', session: aliceSession },
      window.location.origin
    )
    postMessage.mockClear()

    sendMessage(frameWindow, 'https://evil.example.com', {
      type: 'new-api-video-request-session',
    })
    sendMessage({} as Window, window.location.origin, {
      type: 'new-api-video-request-session',
    })
    expect(postMessage).not.toHaveBeenCalled()

    sendMessage(frameWindow, window.location.origin, {
      type: 'new-api-video-request-session',
    })
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'new-api-video-session', session: aliceSession },
      window.location.origin
    )
  })

  it('renews only a trusted iframe session without remounting the canvas', async () => {
    postSession
      .mockResolvedValueOnce({ data: { success: true, data: aliceSession } })
      .mockResolvedValueOnce({
        data: { success: true, data: renewedAliceSession },
      })
    renderStudio()
    const frame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    const frameWindow = getFrameWindow(frame)
    const postMessage = vi.spyOn(frameWindow, 'postMessage')

    sendMessage({} as Window, window.location.origin, {
      type: 'new-api-video-renew-session',
    })
    expect(postSession).toHaveBeenCalledTimes(1)

    sendMessage(frameWindow, window.location.origin, {
      type: 'new-api-video-renew-session',
    })

    await waitFor(() => expect(postSession).toHaveBeenCalledTimes(2))
    expect(screen.getByTitle('Video Studio')).toBe(frame)
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'new-api-video-session', session: renewedAliceSession },
      window.location.origin
    )
  })

  it('returns an actionable session error when host authentication cannot renew', async () => {
    postSession
      .mockResolvedValueOnce({ data: { success: true, data: aliceSession } })
      .mockRejectedValueOnce(new Error('revoked'))
    renderStudio()
    const frame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    const postMessage = vi.spyOn(getFrameWindow(frame), 'postMessage')

    sendMessage(getFrameWindow(frame), window.location.origin, {
      type: 'new-api-video-renew-session',
    })

    await waitFor(() => expect(postSession).toHaveBeenCalledTimes(2))
    expect(postMessage).toHaveBeenCalledWith(
      {
        type: 'new-api-video-session-error',
        message: 'Unable to renew Video Studio. Please sign in again.',
      },
      window.location.origin
    )
  })

  it.each([null, { id: 1, username: '', role: 1 }])(
    'answers renewal with an auth error when the current host identity is missing (%j)',
    async (hostUser) => {
      postSession.mockResolvedValue({
        data: { success: true, data: aliceSession },
      })
      renderStudio()
      const frame = (await screen.findByTitle(
        'Video Studio'
      )) as HTMLIFrameElement
      const frameWindow = getFrameWindow(frame)
      const postMessage = vi.spyOn(frameWindow, 'postMessage')

      act(() => {
        useAuthStore.getState().auth.setUser(hostUser)
        sendMessage(frameWindow, window.location.origin, {
          type: 'new-api-video-renew-session',
        })
        expect(postMessage).toHaveBeenCalledWith(
          {
            type: 'new-api-video-session-error',
            message: 'Unable to renew Video Studio. Please sign in again.',
          },
          window.location.origin
        )
        expect(postSession).toHaveBeenCalledTimes(1)
      })
    }
  )

  it('rejects a renewed session for a different current user', async () => {
    postSession
      .mockResolvedValueOnce({ data: { success: true, data: aliceSession } })
      .mockResolvedValueOnce({ data: { success: true, data: bobSession } })
    renderStudio()
    const frame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    const postMessage = vi.spyOn(getFrameWindow(frame), 'postMessage')

    sendMessage(getFrameWindow(frame), window.location.origin, {
      type: 'new-api-video-renew-session',
    })

    await waitFor(() => expect(postSession).toHaveBeenCalledTimes(2))
    expect(postMessage).toHaveBeenCalledWith(
      {
        type: 'new-api-video-session-error',
        message: 'Unable to renew Video Studio. Please sign in again.',
      },
      window.location.origin
    )
  })

  it('unmounts the old frame and never sends its session after account switching', async () => {
    postSession
      .mockResolvedValueOnce({
        data: { success: true, data: aliceSession },
      })
      .mockResolvedValueOnce({
        data: { success: true, data: bobSession },
      })
    renderStudio()
    const oldFrame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    const oldWindow = getFrameWindow(oldFrame)
    const oldPostMessage = vi.spyOn(oldWindow, 'postMessage')

    fireEvent.load(oldFrame)
    oldPostMessage.mockClear()
    act(() => setUser(2, 'bob'))

    await waitFor(() => expect(postSession).toHaveBeenCalledTimes(2))
    const newFrame = (await screen.findByTitle(
      'Video Studio'
    )) as HTMLIFrameElement
    expect(newFrame).not.toBe(oldFrame)
    const newPostMessage = vi.spyOn(getFrameWindow(newFrame), 'postMessage')

    sendMessage(oldWindow, window.location.origin, {
      type: 'new-api-video-request-session',
    })
    expect(oldPostMessage).not.toHaveBeenCalled()

    fireEvent.load(newFrame)
    expect(newPostMessage).toHaveBeenCalledWith(
      { type: 'new-api-video-session', session: bobSession },
      window.location.origin
    )
    expect(newPostMessage).not.toHaveBeenCalledWith(
      expect.objectContaining({ session: aliceSession }),
      expect.anything()
    )
  })

  it('gives the original canvas the full viewport without an outer navigation bar', async () => {
    postSession.mockResolvedValue({
      data: { success: true, data: aliceSession },
    })

    renderStudio()

    await screen.findByTitle('Video Studio')
    expect(screen.getByRole('main')).toHaveClass('h-dvh')
    expect(screen.queryByRole('banner')).toBeNull()
    expect(screen.queryByRole('link', { name: 'Back to Dashboard' })).toBeNull()
    expect(screen.queryByLabelText('Balance')).toBeNull()
    expect(screen.queryByRole('link', { name: 'Recharge' })).toBeNull()
  })
})
