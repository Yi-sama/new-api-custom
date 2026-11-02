import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { DailyTokenPoolView } from '..'

vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => props.children,
}))

function renderPool() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DailyTokenPoolView />
    </QueryClientProvider>
  )
}

beforeEach(async () => {
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
})

describe('Public shared token pool status', () => {
  it('renders numbers and reset times with the internal Chinese language code', async () => {
    await i18next.changeLanguage('zhCN')
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: true,
          title: 'Community',
          daily_limit: 10000000,
          used_tokens: 100,
          reserved_tokens: 0,
          remaining_tokens: 9999900,
          reset_at: 1792425600,
          reset_timezone: 'Asia/Shanghai',
          exhausted: false,
        },
      },
    })
    renderPool()
    expect(await screen.findByText('9,999,900')).toBeVisible()
    expect(screen.getByText(/Asia\/Shanghai/)).toBeVisible()
  })
  it('shows shared usage, reservations and the next reset without user details', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: true,
          title: 'Community daily pool',
          daily_limit: 10000000,
          used_tokens: 2000000,
          reserved_tokens: 100,
          remaining_tokens: 7999900,
          reset_at: 1792425600,
          reset_timezone: 'Asia/Shanghai',
          exhausted: false,
        },
      },
    })
    renderPool()
    expect(await screen.findByText('Community daily pool')).toBeVisible()
    expect(screen.getByText('7,999,900')).toBeVisible()
    expect(screen.getByText('100')).toBeVisible()
    const capacity = screen.getByRole('progressbar', {
      name: 'Available tokens',
    })
    expect(capacity).toHaveAttribute('aria-valuenow', '7999900')
    expect(capacity).toHaveAttribute('aria-valuemax', '10000000')
    expect(screen.getByText(/Asia\/Shanghai/)).toBeVisible()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
  })

  it('does not show old counters when public visibility is disabled', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { enabled: false } },
    })
    renderPool()
    expect(
      await screen.findByText('Shared token pool is not available')
    ).toBeVisible()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  it('shows exhaustion instead of a misleading available status', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: true,
          title: 'Community',
          daily_limit: 100,
          used_tokens: 100,
          reserved_tokens: 0,
          remaining_tokens: 0,
          reset_at: 1792425600,
          reset_timezone: 'Asia/Shanghai',
          exhausted: true,
        },
      },
    })
    renderPool()
    expect(await screen.findByText('Daily pool exhausted')).toBeVisible()
    expect(screen.queryByText('Available')).not.toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '0'
    )
    expect(screen.getByText('Next daily reset')).toBeVisible()
  })
})
