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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { QQBinding } from '../qq-binding'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }))
const defaultError = vi.fn()

describe('QQ binding', () => {
  beforeEach(() => {
    vi.stubGlobal('React', React)
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: { enabled: true, bound: false, reward_usd: 1 },
      },
    })
  })

  afterEach(() => vi.unstubAllGlobals())

  function showBinding() {
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false, onError: defaultError },
      },
    })
    const update = vi.fn()
    render(
      <QueryClientProvider client={client}>
        <QQBinding userId={1} onUpdate={update} />
      </QueryClientProvider>
    )
    return update
  }

  test('requires a valid QQ number before requesting a code', async () => {
    showBinding()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Bind QQ' })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Bind QQ' }))
    fireEvent.change(screen.getByLabelText('QQ Number'), {
      target: { value: 'invalid' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Generate binding code' })
    )
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Enter a valid QQ number.'
    )
    expect(api.post).not.toHaveBeenCalled()
  })

  test('shows the initiation requirement when the group request is missing', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: false, code: 'QQ_BENEFIT_NEED_START' },
    })
    showBinding()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Bind QQ' })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Bind QQ' }))
    fireEvent.change(screen.getByLabelText('QQ Number'), {
      target: { value: '100001' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Generate binding code' })
    )
    expect(await screen.findByRole('alert')).toHaveTextContent('/领福利')
    expect(defaultError).not.toHaveBeenCalled()
    expect(
      screen.queryByRole('button', { name: 'Copy binding code' })
    ).not.toBeInTheDocument()
  })

  test('copies only the code for direct group submission and clears it after QQ changes', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        data: {
          qq: '100001',
          code: '012345',
          expires_at: Math.floor(Date.now() / 1000) + 300,
          reward_usd: 1,
        },
      },
    })
    showBinding()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Bind QQ' })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Bind QQ' }))
    fireEvent.change(screen.getByLabelText('QQ Number'), {
      target: { value: '100001' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Generate binding code' })
    )
    expect(await screen.findByText('012345')).toBeVisible()
    expect(screen.queryByText('/绑定 012345')).not.toBeInTheDocument()
    expect(screen.getByText('QQ 100001')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Copy binding code' })
    ).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Copy binding code' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('012345'))
    fireEvent.change(screen.getByLabelText('QQ Number'), {
      target: { value: '100002' },
    })
    await waitFor(() =>
      expect(screen.queryByText('012345')).not.toBeInTheDocument()
    )
  })

  test('shows an existing binding without offering another reward', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          enabled: true,
          bound: true,
          qq: '100001',
          reward_usd: 1,
          reward_granted: true,
        },
      },
    })
    showBinding()
    expect(
      await screen.findByText('QQ 100001 · First-binding credit received')
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Bound' })).toBeDisabled()
    expect(api.post).not.toHaveBeenCalled()
  })

  test('keeps a service failure visible instead of treating it as an unbound account', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('network unavailable'))
    showBinding()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'QQ binding is unavailable.'
    )
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
    expect(
      screen.queryByRole('button', { name: 'Bind QQ' })
    ).not.toBeInTheDocument()
  })
})
