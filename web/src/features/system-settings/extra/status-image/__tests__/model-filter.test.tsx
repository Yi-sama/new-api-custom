import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import type { ChannelModelSelection } from '../config'
import { StatusImageModelFilter } from '../model-filter'

const item = {
  name: 'gpt-5',
  status: 'unknown',
  availability: null,
  latency: null,
  checkedAt: 0,
  history: [],
}
const groups = [
  { name: 'A', items: [item, item, { ...item, name: 'claude-sonnet' }] },
  { name: 'B', items: [item] },
]

function Filter() {
  const [value, setValue] = useState<ChannelModelSelection | null>(null)
  return (
    <TooltipProvider>
      <StatusImageModelFilter
        value={value}
        groups={groups}
        groupFilter=''
        onChange={setValue}
      />
    </TooltipProvider>
  )
}

test('model selection is keyboard-operable and search does not remove a selected pair from another group', async () => {
  const user = userEvent.setup()
  render(<Filter />)
  await user.click(screen.getByRole('button', { name: 'Selected models only' }))
  const a = screen.getByRole('checkbox', { name: 'A / gpt-5' })
  expect(screen.getAllByRole('checkbox', { name: 'A / gpt-5' })).toHaveLength(1)
  a.focus()
  await user.keyboard(' ')
  expect(a).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'B / gpt-5' })).not.toBeChecked()
  await user.type(
    screen.getByRole('textbox', { name: 'Search groups or models' }),
    'B'
  )
  await user.click(screen.getByRole('checkbox', { name: 'B / gpt-5' }))
  await user.clear(
    screen.getByRole('textbox', { name: 'Search groups or models' })
  )
  expect(screen.getByRole('checkbox', { name: 'A / gpt-5' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'B / gpt-5' })).toBeChecked()
  await user.click(screen.getByRole('checkbox', { name: 'A / gpt-5' }))
  await user.click(screen.getByRole('checkbox', { name: 'B / gpt-5' }))
  expect(
    screen.getByRole('button', { name: 'Selected models only' })
  ).toHaveAttribute('aria-pressed', 'true')
  await user.type(
    screen.getByRole('textbox', { name: 'Search groups or models' }),
    'missing'
  )
  expect(screen.getByText('No matching models')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'All models' }))
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
})

test('unknown saved models in a current group remain removable while disabled state blocks editing', async () => {
  const user = userEvent.setup()
  const value = [{ group: 'B', model: 'long-model-'.repeat(35) }]
  const view = render(
    <TooltipProvider>
      <StatusImageModelFilter
        value={value}
        groups={[groups[1]]}
        groupFilter='A'
        onChange={() => {}}
      />
    </TooltipProvider>
  )
  expect(
    screen.getByRole('checkbox', { name: `B / ${value[0].model}` })
  ).toBeChecked()
  expect(screen.getByText('Not in current data')).toBeInTheDocument()
  expect(screen.getByText('Outside selected groups')).toBeInTheDocument()
  view.rerender(
    <TooltipProvider>
      <StatusImageModelFilter
        value={value}
        groups={[groups[1]]}
        groupFilter='A'
        onChange={() => {}}
        disabled
      />
    </TooltipProvider>
  )
  expect(
    screen.getByRole('checkbox', { name: `B / ${value[0].model}` })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(screen.getByRole('textbox')).toBeDisabled()
  expect(
    screen.getByRole('button', { name: 'Clear model selection' })
  ).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'All models' }))
  expect(
    screen.getByRole('button', { name: 'Selected models only' })
  ).toHaveAttribute('aria-pressed', 'true')
})

test('a removed group is not reconstructed from saved model selections and can still be cleared explicitly', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  const view = render(
    <TooltipProvider>
      <StatusImageModelFilter
        value={[{ group: 'A', model: 'gpt-5' }]}
        groups={groups}
        groupFilter=''
        onChange={onChange}
      />
    </TooltipProvider>
  )
  expect(screen.getByRole('checkbox', { name: 'A / gpt-5' })).toBeChecked()
  view.rerender(
    <TooltipProvider>
      <StatusImageModelFilter
        value={[{ group: 'A', model: 'gpt-5' }]}
        groups={[groups[1]]}
        groupFilter=''
        onChange={onChange}
      />
    </TooltipProvider>
  )
  expect(
    screen.queryByRole('checkbox', { name: 'A / gpt-5' })
  ).not.toBeInTheDocument()
  expect(screen.getByText('Selected pairs: 1')).toBeInTheDocument()
  expect(onChange).not.toHaveBeenCalled()
  await user.click(
    screen.getByRole('button', { name: 'Clear model selection' })
  )
  expect(onChange).toHaveBeenCalledWith([])
})
