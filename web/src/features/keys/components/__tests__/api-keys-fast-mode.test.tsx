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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { ApiKey } from '../../types'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider, useQuery } =
  await import('@tanstack/react-query')
const { toast } = await import('sonner')
const { api } = await import('@/lib/api')
const { transformApiKeyToFormDefaults, transformFormDataToPayload } =
  await import('../../lib')
const { ApiKeyFastModeCell } = await import('../api-keys-cells')
const { useApiKeysColumns } = await import('../api-keys-columns')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ApiMethod = (
  url: string,
  data?: unknown,
  config?: unknown
) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  put: ApiMethod
}
type ApiKeysQueryData = {
  items: ApiKey[]
  total: number
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPut = apiClient.put
const queryKey = ['keys', 1, 20, '', '', 0]

function createApiKey(overrides: Partial<ApiKey> = {}): ApiKey {
  return {
    id: 7,
    name: 'Primary',
    key: 'masked',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 0,
    accessed_time: 0,
    group: 'default',
    auto_groups: [],
    cross_group_retry: false,
    fast_mode: false,
    fast_mode_available: true,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    ...overrides,
  }
}

function FastModeCellHarness({ initialApiKey }: { initialApiKey: ApiKey }) {
  const { data } = useQuery({
    queryKey,
    queryFn: async (): Promise<ApiKeysQueryData> => ({
      items: [initialApiKey],
      total: 1,
    }),
    initialData: { items: [initialApiKey], total: 1 },
    staleTime: Infinity,
  })

  return <ApiKeyFastModeCell apiKey={data?.items[0] ?? initialApiKey} />
}

function renderFastModeCell(
  initialApiKey = createApiKey()
): InstanceType<typeof QueryClient> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <FastModeCellHarness initialApiKey={initialApiKey} />
      </I18nextProvider>
    </QueryClientProvider>
  )

  return queryClient
}

function ColumnsHarness() {
  const columns = useApiKeysColumns(Date.now())

  return (
    <ol>
      {columns.map((column, index) => {
        const accessorKey =
          'accessorKey' in column ? String(column.accessorKey) : undefined
        const columnId = column.id ?? accessorKey ?? ['column', index].join('-')
        return <li key={columnId}>{columnId}</li>
      })}
    </ol>
  )
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.put = originalPut
  vi.restoreAllMocks()
})

describe('API key fast mode', () => {
  test('hides the switch and multiplier for a group without Fast permission', () => {
    renderFastModeCell(
      createApiKey({
        group: 'images',
        fast_mode: true,
        fast_mode_available: false,
      })
    )

    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(screen.queryByText('1.5x')).not.toBeInTheDocument()
    expect(screen.getByText('--')).toBeInTheDocument()
  })

  test('does not expose Fast when the server has not supplied group eligibility', () => {
    renderFastModeCell(createApiKey({ fast_mode_available: undefined }))

    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(screen.queryByText('1.5x')).not.toBeInTheDocument()
  })

  test('updates the row cache after a successful toggle', async () => {
    const put = vi.fn<ApiMethod>(async (url, data, config) => {
      expect(url).toBe('/api/token/?fast_mode_only=true')
      expect(data).toEqual({ id: 7, fast_mode: true })
      expect(config).toEqual({
        skipBusinessError: true,
        skipErrorHandler: true,
      })
      return {
        data: { success: true, data: createApiKey({ fast_mode: true }) },
      }
    })
    apiClient.put = put
    const queryClient = renderFastModeCell()

    fireEvent.click(screen.getByRole('switch', { name: 'Fast mode' }))

    await waitFor(() =>
      expect(screen.getByRole('switch', { name: 'Fast mode' })).toHaveAttribute(
        'aria-checked',
        'true'
      )
    )
    expect(put).toHaveBeenCalledTimes(1)
    expect(
      queryClient.getQueryData<ApiKeysQueryData>(queryKey)?.items[0]?.fast_mode
    ).toBe(true)
  })

  test('keeps the original state and shows the server error when the toggle fails', async () => {
    apiClient.put = vi.fn<ApiMethod>(async (_url, _data, config) => ({
      data: { success: false, message: 'Fast mode is unavailable' },
      config,
    }))
    const toastError = vi.spyOn(toast, 'error')
    const queryClient = renderFastModeCell()

    fireEvent.click(screen.getByRole('switch', { name: 'Fast mode' }))

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith('Fast mode is unavailable')
    )
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('switch', { name: 'Fast mode' })).toHaveAttribute(
      'aria-checked',
      'false'
    )
    expect(
      queryClient.getQueryData<ApiKeysQueryData>(queryKey)?.items[0]?.fast_mode
    ).toBe(false)
  })

  test('hides a row when the toggle response reports that its group was revoked', async () => {
    apiClient.put = vi.fn<ApiMethod>(async () => ({
      data: {
        success: true,
        data: createApiKey({ fast_mode: true, fast_mode_available: false }),
      },
    }))
    renderFastModeCell()

    fireEvent.click(screen.getByRole('switch', { name: 'Fast mode' }))

    await waitFor(() =>
      expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    )
    expect(screen.queryByText('1.5x')).not.toBeInTheDocument()
  })

  test('disables only the toggled switch while its request is pending', async () => {
    let resolveRequest: ((value: { data: unknown }) => void) | undefined
    apiClient.put = vi.fn(
      () =>
        new Promise<{ data: unknown }>((resolve) => {
          resolveRequest = resolve
        })
    )
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ApiKeyFastModeCell apiKey={createApiKey()} />
          <ApiKeyFastModeCell apiKey={createApiKey({ id: 8 })} />
        </I18nextProvider>
      </QueryClientProvider>
    )

    const fastModeSwitches = screen.getAllByRole('switch', {
      name: 'Fast mode',
    })
    expect(fastModeSwitches).toHaveLength(2)
    const pendingSwitch = fastModeSwitches[0]
    const untouchedSwitch = fastModeSwitches[1]
    if (!pendingSwitch || !untouchedSwitch) {
      throw new Error('Expected two Fast mode switches')
    }
    fireEvent.click(pendingSwitch)

    await waitFor(() =>
      expect(pendingSwitch).toHaveAttribute('aria-disabled', 'true')
    )
    expect(untouchedSwitch).not.toHaveAttribute('aria-disabled', 'true')
    resolveRequest?.({
      data: { success: true, data: createApiKey({ fast_mode: true }) },
    })
    await waitFor(() =>
      expect(pendingSwitch).not.toHaveAttribute('aria-disabled', 'true')
    )
    queryClient.clear()
  })

  test('places Fast mode immediately after Name in the table columns', () => {
    apiClient.get = vi.fn<ApiMethod>(async (url) => {
      expect(url).toBe('/api/user/self/groups')
      return { data: { success: true, data: {} } }
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ColumnsHarness />
        </I18nextProvider>
      </QueryClientProvider>
    )

    const columnIds = screen
      .getAllByRole('listitem')
      .map((item) => item.textContent)
    expect(columnIds.indexOf('fast_mode')).toBe(columnIds.indexOf('name') + 1)
    queryClient.clear()
  })

  test('keeps enabled fast mode in the existing key edit payload', () => {
    const formValues = transformApiKeyToFormDefaults(
      createApiKey({ fast_mode: true })
    )

    expect(transformFormDataToPayload(formValues).fast_mode).toBe(true)
  })
})
