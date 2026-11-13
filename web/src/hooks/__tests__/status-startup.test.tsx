import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { getStatus } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { useStatus } from '../use-status'
import { useSystemConfig } from '../use-system-config'

const { getMock } = vi.hoisted(() => ({ getMock: vi.fn() }))

vi.mock('@/lib/http-client', () => ({ api: { get: getMock } }))

const status = {
  system_name: 'Startup fixture',
  logo: '/logo.png',
  quota_per_unit: 500000,
  usd_exchange_rate: 7,
  quota_display_type: 'CNY',
  display_in_currency: true,
}
const nativeFetch = vi.fn()
let client: QueryClient

function Startup() {
  const config = useSystemConfig({ autoLoad: true })
  const query = useStatus()
  return (
    <>
      <span data-testid='brand'>{config.systemName}</span>
      <span data-testid='currency'>{config.currency.usdExchangeRate}</span>
      <span data-testid='busy'>
        {config.loading || query.loading ? 'loading' : 'ready'}
      </span>
    </>
  )
}

beforeEach(() => {
  localStorage.clear()
  getMock.mockReset()
  nativeFetch.mockReset()
  getMock.mockResolvedValue({ data: { success: true, data: status } })
  nativeFetch.mockResolvedValue({
    ok: true,
    json: async () => ({ success: true, data: status }),
  })
  vi.stubGlobal('fetch', nativeFetch)
  useSystemConfigStore.setState({ loading: true })
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
})

afterEach(() => {
  client.clear()
  vi.unstubAllGlobals()
})

it('shares the startup status request while preserving branding and currency', async () => {
  render(
    <QueryClientProvider client={client}>
      <Startup />
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(screen.getByTestId('busy')).toHaveTextContent('ready')
  )
  expect(screen.getByTestId('brand')).toHaveTextContent('Startup fixture')
  expect(screen.getByTestId('currency')).toHaveTextContent('7')
  expect(getMock.mock.calls.length + nativeFetch.mock.calls.length).toBe(1)
})

it('reuses fresh startup data but refetches invalidated settings', async () => {
  client.setQueryData(['status'], status)
  render(
    <QueryClientProvider client={client}>
      <Startup />
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(screen.getByTestId('busy')).toHaveTextContent('ready')
  )
  expect(getMock.mock.calls.length + nativeFetch.mock.calls.length).toBe(0)

  getMock.mockResolvedValue({
    data: {
      success: true,
      data: { ...status, system_name: 'Updated fixture' },
    },
  })
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['status'] })
  })
  await waitFor(() =>
    expect(screen.getByTestId('brand')).toHaveTextContent('Updated fixture')
  )
  expect(getMock).toHaveBeenCalledTimes(1)
  expect(nativeFetch).not.toHaveBeenCalled()
})

it('rejects unsuccessful status responses instead of caching them as settings', async () => {
  getMock.mockResolvedValue({
    data: { success: false, message: 'Status unavailable' },
  })
  await expect(getStatus()).rejects.toThrow('Status unavailable')
})
