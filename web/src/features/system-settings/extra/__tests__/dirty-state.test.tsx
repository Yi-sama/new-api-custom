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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { NikoGroupSyncSettings } from '../niko-group-sync'

const mocks = vi.hoisted(() => ({
  configValue: '',
  getChannels: vi.fn().mockResolvedValue({
    success: true,
    data: {
      items: [
        { id: 7, name: 'Niko 主渠道', group: 'default,codex-Pro' },
      ],
      total: 1,
      page: 1,
      page_size: 1000,
    },
  }),
  preview: vi.fn().mockResolvedValue({
    success: true,
    message: '',
    data: {
      fetched_at: 1,
      source_url: 'http://127.0.0.1:3010/api/pricing',
      group_ratio: {
        'codex-混池': 0.06,
        'codex-Plus': 0.11,
        'codex-Pro': 0.15,
        Gemini: 0.1,
      },
      usable_group: {},
      items: [],
      remote_count: 4,
      applied: false,
    },
  }),
  save: vi.fn().mockResolvedValue({ success: true, message: '' }),
}))

vi.mock('../../api', () => ({
  getChannels: mocks.getChannels,
  previewNikoGroupSync: mocks.preview,
  saveNikoGroupSync: mocks.save,
}))

vi.mock('@/features/channels/api', () => ({
  getChannels: mocks.getChannels,
}))

vi.mock('../../hooks/use-system-options', () => ({
  useSystemOptions: () => ({
    data: {
      success: true,
      message: '',
      data: [
        {
          key: 'NikoGroupSyncConfig',
          value:
            mocks.configValue ||
            JSON.stringify({
              enabled: false,
              source_url: 'http://127.0.0.1:3010/api/pricing',
              timeout_seconds: 10,
              ratio_adjustment: 0.01,
              base_group_ratio: {
                'codex-混池': 0.04,
                'codex-Plus': 0.11,
                'codex-Pro': 0.15,
                default: 1,
              },
              mappings: [
                {
                  id: 'niko-codex-混池',
                  enabled: true,
                  local_group: 'codex-混池',
                  remote_group: 'codex-混池',
                  ratio_mode: 'remote',
                },
                {
                  id: 'niko-codex-Plus',
                  enabled: true,
                  local_group: 'codex-Plus',
                  remote_group: 'codex-Plus',
                  ratio_mode: 'remote',
                },
                {
                  id: 'niko-codex-Pro',
                  enabled: true,
                  local_group: 'codex-Pro',
                  remote_group: 'codex-Pro',
                  ratio_mode: 'remote',
                },
              ],
            }),
        },
        {
          key: 'GroupRatio',
          value: JSON.stringify({
            'codex-混池': 0.04,
            'codex-Plus': 0.11,
            'codex-Pro': 0.15,
          }),
        },
        {
          key: 'UserUsableGroups',
          value: JSON.stringify({
            'codex-混池': '混池',
            'codex-Plus': 'Plus',
            'codex-Pro': 'Pro',
          }),
        },
      ],
    },
  }),
}))

vi.mock('../../components/form-navigation-guard', () => ({
  FormNavigationGuard: ({ when }: { when: boolean }) => (
    <span data-testid='navigation-guard-state'>{String(when)}</span>
  ),
}))

function renderSettings() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <NikoGroupSyncSettings />
    </QueryClientProvider>
  )
  return queryClient
}

describe('Niko group sync dirty state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.configValue = ''
  })

  test('syncs remote groups into pending mappings without persisting them', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '新增映射' }))
    fireEvent.change(screen.getByLabelText('第 4 行本地分组'), {
      target: { value: 'default' },
    })
    fireEvent.click(screen.getByRole('button', { name: '同步分组' }))
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledTimes(1))

    expect(screen.getByLabelText('第 5 行远端分组')).toHaveValue('Gemini')
    expect(screen.getByLabelText('第 5 行本地分组')).toHaveValue('')
    expect(screen.getByRole('button', { name: '保存设置' })).toBeInTheDocument()

    fireEvent.click(
      screen.getByRole('button', { name: '同步第 5 行远程分组和倍率' })
    )
    expect(screen.getByLabelText('第 5 行本地分组')).toHaveValue('Gemini')
    expect(
      screen.getByRole('spinbutton', { name: '第 5 行当前倍率' })
    ).toHaveValue(0.1)
    expect(
      screen.getByRole('spinbutton', { name: '第 1 行当前倍率' })
    ).toHaveValue(0.04)
    fireEvent.click(screen.getByLabelText('启用第 5 行'))

    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))
    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    expect(mocks.save.mock.calls[0][0].config.mappings).toHaveLength(3)
    expect(mocks.save.mock.calls[0][0].config.mappings).not.toEqual(
      expect.arrayContaining([
        expect.objectContaining({ local_group: 'default' }),
        expect.objectContaining({ local_group: 'Gemini' }),
      ])
    )

    queryClient.clear()
  })

  test('bulk sync enables only pending remote mappings', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '同步分组' }))
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: '一键同步并启用' }))

    expect(screen.getByLabelText('启用第 4 行')).toBeChecked()
    expect(screen.getByLabelText('第 4 行本地分组')).toHaveValue('Gemini')
    expect(
      screen.getByRole('spinbutton', { name: '第 4 行当前倍率' })
    ).toHaveValue(0.1)
    expect(
      screen.getByRole('spinbutton', { name: '第 1 行当前倍率' })
    ).toHaveValue(0.04)

    queryClient.clear()
  })

  test('bulk delete removes only pending remote mappings', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '同步分组' }))
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: '一键删除待配置分组' }))

    expect(screen.queryByLabelText('第 4 行远端分组')).toBeNull()
    expect(screen.getByLabelText('第 1 行本地分组')).toHaveValue('codex-混池')

    queryClient.clear()
  })

  test('shows save only while a local value differs from the saved value', () => {
    const queryClient = renderSettings()
    const currentRatio = screen.getByRole('spinbutton', {
      name: '第 1 行当前倍率',
    })

    expect(screen.queryByRole('button', { name: '保存设置' })).toBeNull()
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'false'
    )

    fireEvent.change(currentRatio, { target: { value: '0.08' } })
    expect(screen.getByRole('button', { name: '保存设置' })).toBeInTheDocument()
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'true'
    )

    fireEvent.change(currentRatio, { target: { value: '0.04' } })
    expect(screen.queryByRole('button', { name: '保存设置' })).toBeNull()
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'false'
    )

    queryClient.clear()
  })

  test('shows channel group list independently from Niko mappings', async () => {
    const queryClient = renderSettings()

    await waitFor(() => expect(mocks.getChannels).toHaveBeenCalledTimes(1))
    expect(await screen.findByText('Niko 主渠道')).toBeInTheDocument()
    expect(screen.getByText('default、codex-Pro')).toBeInTheDocument()

    queryClient.clear()
  })

  test('keeps a saved mapping whose local group is not in UserUsableGroups', () => {
    mocks.configValue = JSON.stringify({
      enabled: false,
      source_url: 'http://127.0.0.1:3010/api/pricing',
      timeout_seconds: 10,
      ratio_adjustment: 0.01,
      base_group_ratio: { Gemini: 0.1 },
      mappings: [
        {
          id: 'niko-Gemini',
          enabled: true,
          local_group: 'Gemini',
          remote_group: 'Gemini',
          ratio_mode: 'remote',
        },
      ],
    })
    const queryClient = renderSettings()

    expect(screen.getByLabelText('第 1 行本地分组')).toHaveValue('Gemini')
    expect(screen.getByLabelText('第 1 行远端分组')).toHaveValue('Gemini')

    queryClient.clear()
  })

  test('deletes an unenabled remote-only row on save', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '同步分组' }))
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))

    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    expect(mocks.save.mock.calls[0][0].config.mappings).toHaveLength(3)

    queryClient.clear()
  })

  test('saves a local-only group ratio without requiring a remote group', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '新增映射' }))
    fireEvent.change(screen.getByLabelText('第 4 行本地分组'), {
      target: { value: 'default' },
    })
    fireEvent.change(
      screen.getByRole('spinbutton', { name: '第 4 行当前倍率' }),
      { target: { value: '0.8' } }
    )
    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))

    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    const payload = mocks.save.mock.calls[0][0]
    expect(payload.config.mappings).toHaveLength(3)
    expect(payload.config.base_group_ratio.default).toBe(1)
    expect(payload.group_ratio).not.toHaveProperty('default')
    expect(payload).not.toHaveProperty('usable_group')

    queryClient.clear()
  })

  test('derives local mapping enablement from group pricing selection', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '新增映射' }))
    fireEvent.change(screen.getByLabelText('第 4 行本地分组'), {
      target: { value: 'default' },
    })
    expect(screen.getByLabelText('启用第 4 行')).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))

    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    const payload = mocks.save.mock.calls[0][0]
    expect(payload.config.mappings).toHaveLength(3)
    expect(payload).not.toHaveProperty('usable_group')

    queryClient.clear()
  })

  test('marks a local mapping disabled when its group is not user selectable', () => {
    mocks.configValue = JSON.stringify({
      enabled: false,
      source_url: 'http://127.0.0.1:3010/api/pricing',
      timeout_seconds: 10,
      ratio_adjustment: 0,
      base_group_ratio: { vip: 1 },
      mappings: [
        {
          id: 'niko-vip',
          enabled: true,
          local_group: 'vip',
          remote_group: 'vip',
          ratio_mode: 'remote',
        },
      ],
    })
    const queryClient = renderSettings()

    expect(screen.getByLabelText('启用第 1 行')).not.toBeChecked()
    expect(screen.getByLabelText('启用第 1 行')).toBeDisabled()

    queryClient.clear()
  })

  test('removes deleted mappings from UserUsableGroups on save', async () => {
    const queryClient = renderSettings()

    for (let index = 0; index < 3; index += 1) {
      fireEvent.click(screen.getByRole('button', { name: '删除第 1 行' }))
    }
    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))

    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    expect(mocks.save.mock.calls[0][0].config.mappings).toEqual([])
    expect(mocks.save.mock.calls[0][0]).not.toHaveProperty('usable_group')

    queryClient.clear()
  })

  test('fetches remote values without becoming dirty and saves only after copying one', async () => {
    const queryClient = renderSettings()

    fireEvent.click(screen.getByRole('button', { name: '获取当前倍率' }))
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledTimes(1))

    expect(screen.getByText('0.06')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '保存设置' })).toBeNull()
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'false'
    )

    fireEvent.click(
      screen.getByRole('button', { name: '同步第 1 行远程分组和倍率' })
    )
    expect(
      screen.getByRole('spinbutton', { name: '第 1 行当前倍率' })
    ).toHaveValue(0.06)
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'true'
    )

    fireEvent.click(screen.getByRole('button', { name: '保存设置' }))
    await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(1))
    expect(mocks.save.mock.calls[0][0].group_ratio['codex-混池']).toBe(0.07)
    expect(
      mocks.save.mock.calls[0][0].config.base_group_ratio['codex-混池']
    ).toBe(0.06)
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '保存设置' })).toBeNull()
    )
    expect(screen.getByTestId('navigation-guard-state')).toHaveTextContent(
      'false'
    )

    queryClient.clear()
  })
})
