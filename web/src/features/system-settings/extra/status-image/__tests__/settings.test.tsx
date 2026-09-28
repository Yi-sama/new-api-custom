import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { StatusImageSettings } from '..'
import { defaultStatusImageConfig, STATUS_IMAGE_OPTION } from '../config'

const mocks = vi.hoisted(() => ({
  load: vi.fn(),
  save: vi.fn(),
  live: vi.fn(),
  error: vi.fn(),
}))
vi.mock('../../../api', () => ({
  getSystemOptions: mocks.load,
  updateSystemOption: mocks.save,
}))
vi.mock('@/features/channel-status/api', () => ({
  getChannelStatusOverview: mocks.live,
}))
vi.mock('@tanstack/react-router', () => ({
  useBlocker: () => ({ status: 'idle' }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: mocks.error } }))

beforeEach(() => {
  mocks.load.mockReset().mockResolvedValue({
    success: true,
    data: [
      {
        key: STATUS_IMAGE_OPTION,
        value: JSON.stringify(defaultStatusImageConfig),
      },
    ],
  })
  mocks.save.mockReset().mockResolvedValue({ success: true })
  mocks.live.mockReset()
  mocks.error.mockReset()
})

async function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={client}>
      <StatusImageSettings />
    </QueryClientProvider>
  )
  await screen.findByLabelText('Report site name')
  return { client, ...view }
}

test('saves both report settings in one option and restores them on remount', async () => {
  const first = await setup()
  fireEvent.change(screen.getByLabelText('Report site name'), {
    target: { value: 'Local test site' },
  })
  fireEvent.change(screen.getByLabelText('Sub image title'), {
    target: { value: 'Account pool' },
  })
  fireEvent.click(
    screen.getByRole('switch', { name: 'Latest request duration' })
  )
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() => expect(mocks.save).toHaveBeenCalledOnce())
  const payload = mocks.save.mock.calls[0][0]
  expect(payload.key).toBe(STATUS_IMAGE_OPTION)
  expect(JSON.parse(payload.value)).toMatchObject({
    site_name: 'Local test site',
    sub_title: 'Account pool',
    show_latency: false,
  })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  )
  first.unmount()
  first.client.clear()
  mocks.load.mockResolvedValue({ success: true, data: [payload] })
  const second = await setup()
  expect(screen.getByLabelText('Report site name')).toHaveValue(
    'Local test site'
  )
  expect(
    screen.getByRole('switch', { name: 'Latest request duration' })
  ).not.toBeChecked()
  second.client.clear()
})

test('rejects a blank site name without issuing a save', async () => {
  const { client } = await setup()
  fireEvent.change(screen.getByLabelText('Report site name'), {
    target: { value: ' ' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() =>
    expect(screen.getByLabelText('Report site name')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  )
  expect(mocks.save).not.toHaveBeenCalled()
  client.clear()
})

test('keeps unsaved edits across background reload and a rejected save', async () => {
  const { client } = await setup()
  fireEvent.change(screen.getByLabelText('Report site name'), {
    target: { value: 'Pending name' },
  })
  mocks.load.mockResolvedValue({
    success: true,
    data: [
      {
        key: STATUS_IMAGE_OPTION,
        value: JSON.stringify({
          ...defaultStatusImageConfig,
          site_name: 'Remote name',
        }),
      },
    ],
  })
  await client.invalidateQueries({ queryKey: ['system-options'] })
  expect(screen.getByLabelText('Report site name')).toHaveValue('Pending name')
  mocks.save.mockResolvedValue({ success: false, message: 'write failed' })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('write failed')
  expect(screen.getByLabelText('Report site name')).toHaveValue('Pending name')
  expect(screen.getByRole('button', { name: 'Discard changes' })).toBeEnabled()
  client.clear()
})

test('shows a live data failure without silently exporting sample data', async () => {
  const { client } = await setup()
  mocks.live.mockRejectedValue(new Error('offline'))
  fireEvent.change(screen.getByLabelText('Preview data'), {
    target: { value: 'live' },
  })
  await screen.findByText('Failed to load channel statistics')
  expect(
    screen.queryByRole('button', { name: 'Download PNG' })
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('tab', { name: 'Sub2API' }))
  expect(screen.getByRole('tab', { name: 'Sub2API' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  client.clear()
})

test('does not expose defaults as an editable form after a settings load failure', async () => {
  mocks.load.mockResolvedValue({ success: false, message: 'denied' })
  const client = new QueryClient()
  render(
    <QueryClientProvider client={client}>
      <StatusImageSettings />
    </QueryClientProvider>
  )
  await screen.findByText('Failed to load status image settings')
  expect(screen.queryByLabelText('Report site name')).not.toBeInTheDocument()
  client.clear()
})
