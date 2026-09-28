import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { InactiveUsersSettings } from '../inactive-users'

const user = {
  id: 7,
  username: 'alice',
  display_name: 'Alice',
  email: 'alice@example.com',
  created_at: 100,
  last_api_call_at: 0,
  api_activity_started_at: 200,
  inactive_days: 45,
}

function pageResponse(overrides: Record<string, unknown> = {}) {
  return {
    success: true,
    data: {
      items: [user],
      total: 1,
      page: 1,
      page_size: 20,
      cutoff: 900,
      observation_count: 3,
      ...overrides,
    },
  }
}

async function renderSettings() {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <InactiveUsersSettings />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

function mockApi(page = pageResponse()) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/inactive-users/settings') {
      return { data: { success: true, data: { days: 30 } } } as never
    }

    return { data: page } as never
  })
  vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, data: { days: 31 } },
  } as never)
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { deleted: 1, skipped: 0, failed: 0 } },
  } as never)
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('inactive API account cleanup', () => {
  it('loads settings and saves only a valid day threshold before refreshing', async () => {
    mockApi()
    await renderSettings()

    const input = await screen.findByLabelText('Inactivity threshold (days)')
    expect(input).toHaveValue(30)
    expect(
      screen.getByRole('button', { name: 'Apply threshold and refresh' })
    ).toBeDisabled()

    await userEvent.clear(input)
    await userEvent.type(input, '0')
    expect(
      screen.getByRole('button', { name: 'Apply threshold and refresh' })
    ).toBeDisabled()

    await userEvent.clear(input)
    await userEvent.type(input, '31')
    expect(
      screen.getByRole('button', { name: 'Apply threshold and refresh' })
    ).toBeEnabled()
    await userEvent.click(
      screen.getByRole('button', { name: 'Apply threshold and refresh' })
    )

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/api/inactive-users/settings', {
        days: 31,
      })
      expect(api.get).toHaveBeenCalledWith('/api/inactive-users', {
        params: { days: 31, page: 1, page_size: 20 },
      })
      expect(
        screen.getByRole('button', { name: 'Apply threshold and refresh' })
      ).toBeDisabled()
    })
  })

  it('captures the list cutoff for a single-account deletion', async () => {
    mockApi()
    await renderSettings()

    await userEvent.click(
      await screen.findByRole('button', { name: 'Delete account alice' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete permanently' })
    )

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/api/inactive-users/delete', {
        days: 30,
        cutoff: 900,
        confirmed: true,
        ids: [7],
      })
    })
  })

  it('disables apply again when the threshold is changed back to the saved value', async () => {
    mockApi()
    await renderSettings()
    const input = await screen.findByLabelText('Inactivity threshold (days)')
    const applyButton = screen.getByRole('button', {
      name: 'Apply threshold and refresh',
    })

    await userEvent.clear(input)
    await userEvent.type(input, '31')
    expect(applyButton).toBeEnabled()

    await userEvent.clear(input)
    await userEvent.type(input, '30')
    expect(applyButton).toBeDisabled()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('shows a deletion failure inside the confirmation and refreshes candidates', async () => {
    mockApi()
    vi.mocked(api.post).mockRejectedValueOnce(
      new Error('Cleanup connection lost')
    )
    await renderSettings()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Delete account alice' })
    )
    vi.mocked(api.get).mockClear()
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete permanently' })
    )
    expect(
      (await screen.findAllByText('Cleanup connection lost')).length
    ).toBeGreaterThan(0)
    await waitFor(() =>
      expect(api.get).toHaveBeenCalledWith('/api/inactive-users', {
        params: { days: 30, page: 1, page_size: 20 },
      })
    )
  })

  it('shows an empty state without allowing bulk deletion', async () => {
    mockApi(pageResponse({ total: 0, items: [] }))
    await renderSettings()
    expect(
      await screen.findByText('No eligible inactive accounts found.')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Delete all 0 matching accounts' })
    ).toBeDisabled()
  })

  it('keeps account text within a horizontally scrollable narrow-screen table', async () => {
    mockApi(
      pageResponse({
        items: [
          {
            ...user,
            username: 'long-account-name',
            email: 'a-very-long-address@example.test',
          },
        ],
      })
    )
    await renderSettings()
    const username = await screen.findByText('long-account-name', {
      exact: true,
    })
    expect(username.closest('td')).toHaveClass('whitespace-normal')
    expect(screen.getByRole('table')).toHaveClass('min-w-[960px]')
    expect(screen.getByRole('table').parentElement).toHaveClass(
      'overflow-x-auto'
    )
  })

  it('captures the list cutoff for all matching accounts across pages', async () => {
    mockApi(pageResponse({ total: 23, cutoff: 901 }))
    await renderSettings()

    await userEvent.click(
      await screen.findByRole('button', {
        name: 'Delete all 23 matching accounts',
      })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete permanently' })
    )

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/api/inactive-users/delete', {
        days: 30,
        cutoff: 901,
        confirmed: true,
        all: true,
      })
    })
  })

  it('does not mutate when the deletion confirmation is cancelled', async () => {
    mockApi()
    await renderSettings()

    await userEvent.click(
      await screen.findByRole('button', { name: 'Delete account alice' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(api.post).not.toHaveBeenCalled()
  })

  it('blocks deletion while the threshold has unapplied changes', async () => {
    mockApi()
    await renderSettings()

    const input = await screen.findByLabelText('Inactivity threshold (days)')
    await userEvent.clear(input)
    await userEvent.type(input, '31')

    expect(
      screen.getByRole('button', { name: 'Delete account alice' })
    ).toBeDisabled()
    expect(
      screen.getByText(
        'Saved threshold is stale. Apply and refresh before deleting.'
      )
    ).toBeInTheDocument()
  })

  it('shows accurate partial cleanup counts', async () => {
    mockApi()
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: { deleted: 1, skipped: 2, failed: 3 } },
    } as never)
    await renderSettings()

    await userEvent.click(
      await screen.findByRole('button', { name: 'Delete account alice' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Delete permanently' })
    )

    expect(
      await screen.findByText('Deleted 1 account(s), skipped 2, failed 3.')
    ).toBeInTheDocument()
  })

  it('uses a 20-account page size and advances pagination', async () => {
    mockApi(pageResponse({ total: 21 }))
    await renderSettings()

    expect(await screen.findByText('Page 1 of 2')).toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Go to next page' })
    )

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/api/inactive-users', {
        params: { days: 30, page: 2, page_size: 20 },
      })
    })
  })
})
