import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { DailyTokenPoolSettings } from '../daily-token-pool'

vi.mock('@tanstack/react-router', async (original) => ({
  ...(await original<typeof import('@tanstack/react-router')>()),
  useBlocker: () => ({ status: 'idle' }),
}))

const config = {
  enabled: true,
  title: 'Community',
  daily_limit: 10000000,
  reset_timezone: 'Asia/Shanghai',
  groups: ['community'],
  channel_ids: [7],
  public_visible: true,
}

function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DailyTokenPoolSettings />
    </QueryClientProvider>
  )
}

beforeEach(async () => {
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/group/'
          ? ['community', 'paid']
          : {
              config,
              channels: [{ id: 7, name: 'Community upstream' }],
            },
    },
  }))
  vi.spyOn(api, 'put').mockImplementation(async (_url, value) => ({
    data: { success: true, data: value },
  }))
})

afterEach(() => {
  i18next.removeResourceBundle('zhCN', 'translation')
})

describe('Administrator shared token pool settings', () => {
  it('groups quota, routing and public display controls separately', async () => {
    renderSettings()
    const allowance = await screen.findByRole('group', {
      name: 'Allowance and reset',
    })
    expect(within(allowance).getByRole('spinbutton')).toHaveValue(10000000)
    expect(
      within(allowance).getByRole('textbox', { name: 'Reset timezone' })
    ).toHaveValue('Asia/Shanghai')
    expect(
      within(screen.getByRole('group', { name: 'Routing scope' })).getAllByRole(
        'combobox'
      )
    ).toHaveLength(2)
    expect(
      within(screen.getByRole('group', { name: 'Public display' })).getByRole(
        'switch',
        { name: 'Public pool status' }
      )
    ).toBeChecked()
  })

  it('uses translated group and channel names in the Chinese interface', async () => {
    i18next.addResourceBundle('zhCN', 'translation', zh.translation)
    await i18next.changeLanguage('zhCN')
    renderSettings()
    expect(
      await screen.findByRole('combobox', { name: '所有分组' })
    ).toBeVisible()
    expect(screen.getByRole('combobox', { name: '全部渠道' })).toBeVisible()
  })

  it('normalizes existing configurations without free usage to false when saving', async () => {
    renderSettings()
    const allowance = await screen.findByRole('spinbutton', {
      name: 'Daily token allowance',
    })
    await userEvent.clear(allowance)
    await userEvent.type(allowance, '25000000')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith('/api/daily-token-pool/admin', {
        ...config,
        daily_limit: 25000000,
        free_usage: false,
      })
    )
  })

  it('sends enabled free shared usage to the administrator API', async () => {
    renderSettings()
    const freeUsage = await screen.findByRole('switch', {
      name: 'Free shared usage (do not deduct personal balance)',
    })
    expect(freeUsage).not.toBeChecked()
    await userEvent.click(freeUsage)
    expect(freeUsage).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith('/api/daily-token-pool/admin', {
        ...config,
        free_usage: true,
      })
    )
  })

  it('does not submit an invalid timezone', async () => {
    renderSettings()
    const timezone = await screen.findByRole('textbox', {
      name: 'Reset timezone',
    })
    await userEvent.clear(timezone)
    await userEvent.type(timezone, 'invalid-zone')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByRole('alert')).toBeVisible()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('reset restores the original allowance and visibility without saving', async () => {
    renderSettings()
    const allowance = await screen.findByRole('spinbutton', {
      name: 'Daily token allowance',
    })
    await userEvent.clear(allowance)
    await userEvent.type(allowance, '200')
    await userEvent.click(
      screen.getByRole('switch', { name: 'Public pool status' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Reset' }))
    expect(allowance).toHaveValue(10000000)
    expect(
      screen.getByRole('switch', { name: 'Public pool status' })
    ).toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(api.put).not.toHaveBeenCalled()
  })
})
