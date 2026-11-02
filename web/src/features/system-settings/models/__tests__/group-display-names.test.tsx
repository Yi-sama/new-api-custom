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
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { RatioSettingsCard } from '../ratio-settings-card'

const put = vi.fn<typeof api.put>()
const actions = document.createElement('div')
let queryClient: QueryClient

function renderSettings(names = '{}') {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  document.body.append(actions)
  return render(
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actions}>
        <RatioSettingsCard
          visibleTabs={['groups']}
          modelDefaults={{
            ModelPrice: '{}',
            ModelRatio: '{}',
            CacheRatio: '{}',
            CreateCacheRatio: '{}',
            CompletionRatio: '{}',
            ImageRatio: '{}',
            AudioRatio: '{}',
            AudioCompletionRatio: '{}',
            ExposeRatioEnabled: false,
            BillingMode: '{}',
            BillingExpr: '{}',
          }}
          groupDefaults={{
            GroupRatio: '{"default":0.2}',
            GroupRatioAddition: '{}',
            TopupGroupRatio: '{}',
            UserUsableGroups: '{"default":"Testing"}',
            GroupDisplayNames: names,
            GroupGroupRatio: '{}',
            AutoGroups: '["default"]',
            MaxTokenAutoGroups: 5,
            DefaultUseAutoGroup: false,
            GroupSpecialUsableGroup: '{}',
          }}
          toolPricesDefault='{}'
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

function editNames(value: string) {
  fireEvent.input(
    screen.getByRole('textbox', { name: 'Group display names' }),
    {
      target: { value },
    }
  )
}

function saveNames() {
  fireEvent.click(screen.getByRole('button', { name: 'Save group ratios' }))
}

beforeEach(() => {
  vi.spyOn(api, 'put').mockImplementation(put)
  put.mockResolvedValue({ data: { success: true, message: '' } })
})

afterEach(() => {
  cleanup()
  queryClient?.clear()
  actions.replaceChildren()
  actions.remove()
})

describe('group display names in billing settings', () => {
  test('loads saved names and saves only the display mapping without renaming groups', async () => {
    renderSettings('{"default":"GPT"}')
    const editor = screen.getByRole('textbox', { name: 'Group display names' })
    expect(editor).toHaveValue('{\n  "default": "GPT"\n}')

    editNames('{"default":"Claude","auto":"All"}')
    saveNames()

    await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'GroupDisplayNames',
      value: '{"default":"Claude","auto":"All"}',
    })
  })

  test('keeps the same mapping when switching between visual and JSON modes', () => {
    renderSettings()
    editNames('{"default":"GPT"}')
    fireEvent.click(screen.getByRole('button', { name: 'Switch to JSON' }))
    expect(
      screen.getByRole('textbox', { name: 'Group display names' })
    ).toHaveValue('{"default":"GPT"}')
    fireEvent.click(screen.getByRole('button', { name: 'Switch to Visual' }))
    expect(
      screen.getByRole('textbox', { name: 'Group display names' })
    ).toHaveValue('{"default":"GPT"}')
  })

  test.each(['null', '[]', '{"default":1}', '{"default":null}', '{" ":"GPT"}'])(
    'rejects invalid display mapping %s before contacting the server',
    async (value) => {
      renderSettings()
      editNames(value)
      saveNames()

      await waitFor(() =>
        expect(
          screen.getByRole('textbox', { name: 'Group display names' })
        ).toHaveAttribute('aria-invalid', 'true')
      )
      expect(
        screen.getByText(
          'Expected a JSON object mapping group identifiers to display names'
        )
      ).toBeInTheDocument()
      expect(put).not.toHaveBeenCalled()
    }
  )

  test('saves an empty object to remove all custom display names', async () => {
    renderSettings('{"default":"GPT"}')
    editNames('{}')
    saveNames()
    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/option/', {
        key: 'GroupDisplayNames',
        value: '{}',
      })
    )
  })

  test('keeps a rejected save pending so the same mapping can be retried', async () => {
    put.mockResolvedValueOnce({
      data: { success: false, message: 'Saving failed' },
    })
    renderSettings()
    editNames('{"default":"GPT"}')
    saveNames()
    await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Save group ratios' })
      ).toBeEnabled()
    )
    saveNames()
    await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  })
})
