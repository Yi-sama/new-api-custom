import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen, waitFor } from '@testing-library/react'
import { useRef } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { VideoStudioAccountBridge } from '../account-bridge'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }))

const profile = {
  id: 1,
  username: 'alice',
  display_name: 'Alice',
  group: 'default',
  quota: 500000,
  used_quota: 125000,
  request_count: 2,
}

function Studio() {
  const frameRef = useRef<HTMLIFrameElement>(null)
  const id = useAuthStore((state) => state.auth.user?.id)
  return (
    <>
      <iframe key={`frame-${id}`} ref={frameRef} title='Canvas' sandbox='' />
      <VideoStudioAccountBridge key={`bridge-${id}`} frameRef={frameRef} />
    </>
  )
}

function renderBridge() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: Studio }),
    history: createMemoryHistory(),
  })
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

function message(
  source: Window,
  data: unknown,
  origin = window.location.origin
) {
  act(() =>
    window.dispatchEvent(new MessageEvent('message', { origin, source, data }))
  )
}

async function observeFrame() {
  const frame = await screen.findByTitle<HTMLIFrameElement>('Canvas')
  if (!frame.contentWindow) throw new Error('Frame not available')
  return {
    source: frame.contentWindow,
    post: vi.spyOn(frame.contentWindow, 'postMessage'),
  }
}

describe('Native Canvas account bridge', () => {
  beforeEach(() => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
    useAuthStore.getState().auth.reset()
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    vi.mocked(api.get)
      .mockReset()
      .mockResolvedValue({ data: { success: true, data: profile } })
  })

  it('sends server balances in site currency and synchronizes the shared dashboard cache', async () => {
    renderBridge()
    const { source, post } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
    message(source, { type: 'new-api-video-ready' })
    expect(post).toHaveBeenLastCalledWith(
      {
        type: 'new-api-video-account',
        account: {
          user_id: 1,
          username: 'alice',
          display_name: 'Alice',
          group: 'default',
          balance: formatQuota(profile.quota),
          used: formatQuota(profile.used_quota),
          loading: false,
        },
      },
      window.location.origin
    )
    expect(screen.queryByRole('banner')).toBeNull()
  })

  it('reloads authoritative balances after trusted consumption events and window focus', async () => {
    renderBridge()
    const { source, post } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: { ...profile, quota: 250000 } },
    })
    message(source, {
      type: 'new-api-video-account-changed',
      balance: 'forged',
    })
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(250000)
    )
    expect(post).toHaveBeenLastCalledWith(
      expect.objectContaining({
        account: expect.objectContaining({ balance: formatQuota(250000) }),
      }),
      window.location.origin
    )
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: profile },
    })
    act(() => window.dispatchEvent(new Event('focus')))
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
  })

  it('rejects foreign frames and origins for refresh and logout actions', async () => {
    renderBridge()
    const { source } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
    vi.mocked(api.get).mockClear()
    for (const type of [
      'new-api-video-account-changed',
      'new-api-video-account-action',
    ]) {
      message(source, { type, action: 'sign-out' }, 'https://evil.example')
      message({} as Window, { type, action: 'sign-out' })
    }
    expect(api.get).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).toBeNull()
    message(source, {
      type: 'new-api-video-account-action',
      action: 'sign-out',
    })
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(
      'Are you sure'
    )
  })

  it('clears the displayed amount on failed refresh instead of showing stale credit', async () => {
    renderBridge()
    const { source, post } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
    vi.mocked(api.get).mockRejectedValue(new Error('offline'))
    message(source, { type: 'new-api-video-account-changed' })
    await waitFor(() =>
      expect(post).toHaveBeenLastCalledWith(
        expect.objectContaining({
          account: expect.objectContaining({
            balance: null,
            used: null,
            loading: false,
          }),
        }),
        window.location.origin
      )
    )
  })

  it('marks balances as loading while authoritative refresh is still pending', async () => {
    renderBridge()
    const { source, post } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(profile.quota)
    )
    let resolveRefresh!: (value: unknown) => void
    vi.mocked(api.get).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRefresh = resolve
        })
    )
    message(source, { type: 'new-api-video-account-changed' })
    await waitFor(() =>
      expect(post).toHaveBeenLastCalledWith(
        expect.objectContaining({
          account: expect.objectContaining({
            balance: null,
            used: null,
            loading: true,
          }),
        }),
        window.location.origin
      )
    )
    await act(async () =>
      resolveRefresh({ data: { success: true, data: profile } })
    )
    await waitFor(() =>
      expect(post).toHaveBeenLastCalledWith(
        expect.objectContaining({
          account: expect.objectContaining({
            balance: formatQuota(profile.quota),
            loading: false,
          }),
        }),
        window.location.origin
      )
    )
  })

  it('never hands a previous account response to a newly signed-in user', async () => {
    let resolveOld!: (value: unknown) => void
    vi.mocked(api.get).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOld = resolve
        })
    )
    renderBridge()
    await observeFrame()
    await waitFor(() => expect(api.get).toHaveBeenCalledTimes(1))
    const bob = { ...profile, id: 2, username: 'bob', quota: 750000 }
    vi.mocked(api.get).mockResolvedValue({ data: { success: true, data: bob } })
    act(() =>
      useAuthStore.getState().auth.setUser({ id: 2, username: 'bob', role: 1 })
    )
    const { source, post } = await observeFrame()
    await waitFor(() =>
      expect(useAuthStore.getState().auth.user?.quota).toBe(bob.quota)
    )
    await act(async () =>
      resolveOld({ data: { success: true, data: profile } })
    )
    message(source, { type: 'new-api-video-ready' })
    expect(post).toHaveBeenLastCalledWith(
      expect.objectContaining({
        account: expect.objectContaining({
          user_id: 2,
          balance: formatQuota(bob.quota),
        }),
      }),
      window.location.origin
    )
    expect(useAuthStore.getState().auth.user?.id).toBe(2)
  })
})
