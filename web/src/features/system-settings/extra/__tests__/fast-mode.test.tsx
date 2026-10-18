import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { FastModeSettings } from '../fast-mode'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useBlocker: () => ({ status: 'idle' }),
}))

function renderSettings(groups: string[] = []) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <FastModeSettings FastModeAllowedGroups={groups} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['gpt-paid', 'Gemini', 'auto'] },
  })
  vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, message: '' },
  })
  vi.spyOn(toast, 'success').mockImplementation(() => '')
  vi.spyOn(toast, 'error').mockImplementation(() => '')
})

describe('Fast group administrator settings', () => {
  it('defaults to no allowed groups and does not offer the auto routing placeholder', async () => {
    renderSettings()
    expect(
      await screen.findByRole('checkbox', { name: 'gpt-paid' })
    ).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Gemini' })).not.toBeChecked()
    expect(
      screen.queryByRole('checkbox', { name: 'auto' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })

  it('saves only the chosen group and disables changes while saving', async () => {
    let complete: (value: unknown) => void = () => undefined
    vi.mocked(api.put).mockReturnValueOnce(
      new Promise((resolve) => {
        complete = resolve
      })
    )
    renderSettings()
    const checkbox = await screen.findByRole('checkbox', { name: 'gpt-paid' })
    await userEvent.click(checkbox)
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(checkbox).toHaveAttribute('aria-disabled', 'true')
    )
    await userEvent.click(checkbox)
    expect(checkbox).toBeChecked()
    expect(api.put).toHaveBeenCalledWith(
      '/api/option/',
      { key: 'FastModeAllowedGroups', value: '["gpt-paid"]' },
      { skipBusinessError: true, skipErrorHandler: true }
    )
    complete({ data: { success: true } })
    await waitFor(() =>
      expect(checkbox).not.toHaveAttribute('aria-disabled', 'true')
    )
    expect(checkbox).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Gemini' })).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })

  it('can revoke every group by saving an empty list', async () => {
    renderSettings(['gpt-paid'])
    await userEvent.click(
      await screen.findByRole('checkbox', { name: 'gpt-paid' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith(
        '/api/option/',
        { key: 'FastModeAllowedGroups', value: '[]' },
        { skipBusinessError: true, skipErrorHandler: true }
      )
    )
  })

  it('keeps unsaved selection after a rejected save and reset restores saved groups', async () => {
    vi.mocked(api.put).mockResolvedValueOnce({
      data: { success: false, message: 'Policy save rejected' },
    })
    renderSettings()
    const checkbox = await screen.findByRole('checkbox', { name: 'gpt-paid' })
    await userEvent.click(checkbox)
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Policy save rejected')
    )
    expect(checkbox).toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Reset' }))
    expect(checkbox).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })

  it('reports a group load failure and cannot overwrite policy from an incomplete list', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('Offline'))
    renderSettings(['gpt-paid'])
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Failed to load groups'
    )
    expect(screen.getByRole('checkbox', { name: 'gpt-paid' })).toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(api.put).not.toHaveBeenCalled()
  })
})
