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
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { GroupDisplayNamesSettings } from '../group-display-names'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useBlocker: () => ({ status: 'idle' }),
}))

const put = vi.fn<typeof api.put>()
let queryClient: QueryClient

function renderSettings(names = '{}') {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <GroupDisplayNamesSettings GroupDisplayNames={names} />
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

async function saveNames() {
  const button = screen.getByRole('button', { name: 'Save changes' })
  await waitFor(() => expect(button).toBeEnabled())
  fireEvent.click(button)
}

beforeEach(() => {
  vi.spyOn(api, 'put').mockImplementation(put)
  vi.spyOn(toast, 'error').mockImplementation(() => '')
  vi.spyOn(toast, 'success').mockImplementation(() => '')
  put.mockResolvedValue({ data: { success: true, message: '' } })
})

afterEach(() => {
  cleanup()
  queryClient?.clear()
})

describe('group display names in extra settings', () => {
  test('loads saved names and saves only the display mapping without renaming groups', async () => {
    renderSettings('{"default":"GPT"}')
    const editor = screen.getByRole('textbox', { name: 'Group display names' })
    expect(editor).toHaveValue('{\n  "default": "GPT"\n}')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()

    editNames('{"default":"Claude","auto":"All"}')
    await saveNames()

    await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'GroupDisplayNames',
      value: '{"default":"Claude","auto":"All"}',
    })
  })

  test('reset discards edits and restores the saved display mapping', async () => {
    renderSettings('{"default":"GPT"}')
    editNames('{"default":"Claude"}')
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Reset' })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Reset' }))
    expect(
      screen.getByRole('textbox', { name: 'Group display names' })
    ).toHaveValue('{\n  "default": "GPT"\n}')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(put).not.toHaveBeenCalled()
  })

  test.each([
    '',
    '{',
    'null',
    '[]',
    '{"default":1}',
    '{"default":null}',
    '{" ":"GPT"}',
  ])(
    'rejects invalid display mapping %s before contacting the server',
    async (value) => {
      renderSettings()
      editNames(value)
      await saveNames()

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
    await saveNames()
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
    await saveNames()
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Saving failed')
    )
    expect(
      screen.getByRole('textbox', { name: 'Group display names' })
    ).toHaveValue('{"default":"GPT"}')
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
    )
    await saveNames()
    await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  })

  test('disables editing while saving and marks a successful save as clean', async () => {
    let complete!: (response: { data: { success: boolean } }) => void
    put.mockReturnValueOnce(
      new Promise((resolve) => {
        complete = resolve
      })
    )
    renderSettings()
    editNames('{"default":"GPT"}')
    await saveNames()

    const editor = screen.getByRole('textbox', { name: 'Group display names' })
    await waitFor(() => expect(editor).toBeDisabled())
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()

    complete({ data: { success: true } })
    await waitFor(() => expect(editor).toBeEnabled())
    expect(editor).toHaveValue('{"default":"GPT"}')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })
})
