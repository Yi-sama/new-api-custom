import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { StatusImageGroupFilter } from '../group-filter'

function Filter() {
  const [value, setValue] = useState('Custom')
  return (
    <StatusImageGroupFilter
      id='channel-filter'
      label='Channel groups'
      inputLabel='Channel filter'
      value={value}
      options={['Plus', 'Pro']}
      onChange={setValue}
    />
  )
}

test('group menu excludes stale selections, selects known groups and clears only on explicit all', async () => {
  const user = userEvent.setup()
  render(<Filter />)
  const menu = screen.getByRole('button', { name: 'Channel groups' })
  await user.click(menu)
  expect(menu).toHaveAttribute('aria-expanded', 'true')
  expect(
    screen.queryByRole('checkbox', { name: 'Custom' })
  ).not.toBeInTheDocument()
  expect(screen.getByLabelText('Channel filter')).toHaveValue('Custom')
  expect(screen.getByRole('checkbox', { name: 'All groups' })).not.toBeChecked()
  await user.click(screen.getByRole('checkbox', { name: 'Plus' }))
  expect(screen.getByLabelText('Channel filter')).toHaveValue('Custom,Plus')
  await user.click(screen.getByRole('checkbox', { name: 'All groups' }))
  expect(screen.getByLabelText('Channel filter')).toHaveValue('')
  expect(screen.getByRole('checkbox', { name: 'All groups' })).toBeChecked()
  await user.keyboard('{Escape}')
  expect(menu).toHaveAttribute('aria-expanded', 'false')
  expect(menu).toHaveFocus()
})

test('a deleted selected group disappears when the current options change without broadening the saved filter', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  const props = {
    id: 'channel-filter',
    label: 'Channel groups',
    inputLabel: 'Channel filter',
    value: 'Deleted pool',
    onChange,
  }
  const view = render(
    <StatusImageGroupFilter
      {...props}
      options={['Current pool', 'Deleted pool']}
    />
  )
  await user.click(screen.getByRole('button', { name: 'Channel groups' }))
  expect(screen.getByRole('checkbox', { name: 'Deleted pool' })).toBeChecked()

  view.rerender(
    <StatusImageGroupFilter {...props} options={['Current pool']} />
  )

  expect(
    screen.queryByRole('checkbox', { name: 'Deleted pool' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('checkbox', { name: 'All groups' })).not.toBeChecked()
  expect(screen.getByLabelText('Channel filter')).toHaveValue('Deleted pool')
  expect(onChange).not.toHaveBeenCalled()
})

test('custom typing and disabled state remain available without a populated group list', async () => {
  const user = userEvent.setup()
  const view = render(<Filter />)
  const input = screen.getByLabelText('Channel filter')
  await user.clear(input)
  await user.type(input, 'Unknown pool')
  expect(input).toHaveValue('Unknown pool')
  view.rerender(
    <StatusImageGroupFilter
      id='channel-filter'
      label='Channel groups'
      inputLabel='Channel filter'
      value='Unknown pool'
      options={[]}
      onChange={() => {}}
      disabled
    />
  )
  expect(screen.getByLabelText('Channel filter')).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Channel groups' })).toBeDisabled()
})
