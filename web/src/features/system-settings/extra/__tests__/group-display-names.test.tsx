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
  within,
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
  queryClient.setQueryData(['system-options'], {
    success: true,
    data: [
      {
        key: 'UserUsableGroups',
        value: '{"default":"Standard","vip":"Premium","cn":"Local models"}',
      },
      { key: 'GroupRatio', value: '{"default":0.2,"vip":0,"cn":0.15}' },
    ],
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <GroupDisplayNamesSettings GroupDisplayNames={names} />
    </QueryClientProvider>
  )
}

function editName(value: string, index = 0) {
  fireEvent.change(screen.getAllByRole('textbox')[index], {
    target: { value },
  })
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
  test('loads legacy mappings into rows and offers usable groups with description, ratio and auto', () => {
    renderSettings('{"default":"GPT"}')
    expect(screen.getByRole('textbox')).toHaveValue('GPT')
    const select = screen.getByRole('combobox')
    expect(select).toHaveValue('default')
    expect(
      within(select).getByRole('option', { name: /default.*Standard.*0.2/ })
    ).toBeInTheDocument()
    expect(
      within(select).getByRole('option', { name: /vip.*Premium.*Free/ })
    ).toBeInTheDocument()
    expect(
      within(select).getByRole('option', { name: /auto/ })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move down' })).toBeDisabled()
  })

  test('saves trimmed names and the visible row order as an array without renaming groups', async () => {
    renderSettings(
      '[{"group":"default","name":"GPT"},{"group":"vip","name":"Claude"}]'
    )
    editName(' GPT  ')
    fireEvent.click(screen.getAllByRole('button', { name: 'Move down' })[0])
    expect(screen.getAllByRole('textbox')[0]).toHaveValue('Claude')
    await saveNames()
    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/option/', {
        key: 'GroupDisplayNames',
        value:
          '[{"group":"vip","name":"Claude"},{"group":"default","name":"GPT"}]',
      })
    )
    expect(screen.getAllByRole('textbox')[1]).toHaveValue('GPT')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })

  test('empty settings can add template rows with no preselected groups', () => {
    renderSettings()
    fireEvent.click(
      screen.getByRole('button', {
        name: 'GPT / Claude / Gemini / Local models',
      })
    )
    expect(
      screen
        .getAllByRole('textbox')
        .map((input) => (input as HTMLInputElement).value)
    ).toEqual(['GPT', 'Claude', 'Gemini', '国模'])
    for (const select of screen.getAllByRole('combobox')) {
      expect(select).toHaveValue('')
    }
    fireEvent.change(screen.getAllByRole('combobox')[0], {
      target: { value: 'auto' },
    })
    expect(screen.getAllByRole('combobox')[0]).toHaveValue('auto')
    fireEvent.click(screen.getAllByRole('button', { name: 'Move up' })[1])
    expect(screen.getAllByRole('textbox')[0]).toHaveValue('Claude')
  })

  test('adding an empty row reports required fields and does not contact the server', async () => {
    renderSettings()
    fireEvent.click(screen.getByRole('button', { name: 'Add row' }))
    await saveNames()
    await waitFor(() => {
      expect(screen.getByRole('textbox')).toHaveAttribute(
        'aria-invalid',
        'true'
      )
      expect(screen.getByRole('combobox')).toHaveAttribute(
        'aria-invalid',
        'true'
      )
    })
    expect(put).not.toHaveBeenCalled()
  })

  test.each([
    [
      '[{"group":"default","name":"GPT"},{"group":"default","name":"Claude"}]',
      'Each group can only appear once',
    ],
    [
      '[{"group":"default","name":"GPT"},{"group":"vip","name":" GPT "}]',
      'Display names must be unique',
    ],
    [
      JSON.stringify([{ group: 'default', name: 'x'.repeat(201) }]),
      'Display names must be at most 200 characters',
    ],
  ])('rejects invalid rows %s before saving', async (rows, error) => {
    renderSettings(rows)
    editName(
      `${(screen.getAllByRole('textbox')[0] as HTMLInputElement).value} `
    )
    await saveNames()
    expect(await screen.findAllByText(error)).not.toHaveLength(0)
    expect(put).not.toHaveBeenCalled()
  })

  test('reset discards edits and restores the saved display mapping', async () => {
    renderSettings('{"default":"GPT"}')
    editName('Claude')
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Reset' })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Reset' }))
    expect(screen.getByRole('textbox')).toHaveValue('GPT')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    expect(put).not.toHaveBeenCalled()
  })

  test('keeps a deleted group visible for editing and can remove all rows', async () => {
    renderSettings('{"deleted":"GPT"}')
    expect(screen.getByRole('combobox')).toHaveValue('deleted')
    fireEvent.click(screen.getByRole('button', { name: 'Delete row' }))
    await saveNames()
    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/option/', {
        key: 'GroupDisplayNames',
        value: '[]',
      })
    )
  })

  test('keeps a rejected save pending so the same mapping can be retried', async () => {
    put.mockResolvedValueOnce({
      data: { success: false, message: 'Saving failed' },
    })
    renderSettings()
    fireEvent.click(screen.getByRole('button', { name: 'Add row' }))
    editName('GPT')
    fireEvent.change(screen.getByRole('combobox'), {
      target: { value: 'default' },
    })
    await saveNames()
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Saving failed')
    )
    expect(screen.getByRole('textbox')).toHaveValue('GPT')
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
    renderSettings('{"default":"GPT"}')
    editName('Claude')
    await saveNames()

    const editor = screen.getByRole('textbox')
    await waitFor(() => expect(editor).toBeDisabled())
    expect(screen.getByRole('combobox')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Add row' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete row' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()

    complete({ data: { success: true } })
    await waitFor(() => expect(screen.getByRole('textbox')).toBeEnabled())
    expect(screen.getByRole('textbox')).toHaveValue('Claude')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })
})
