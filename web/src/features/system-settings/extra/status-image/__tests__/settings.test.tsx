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
vi.mock('../api', () => ({
  getStatusImageOverview: mocks.live,
}))
vi.mock('@tanstack/react-router', () => ({
  useBlocker: () => ({ status: 'idle' }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: mocks.error } }))

beforeEach(() => {
  vi.spyOn(window, 'matchMedia').mockReturnValue({
    matches: true,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  } as unknown as MediaQueryList)
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
  mocks.live.mockReset().mockResolvedValue({
    success: true,
    data: {
      config: defaultStatusImageConfig,
      channel: { enabled: true, mode: 'user', checked_at: 1, groups: [] },
      sub: { status: 'connected', checked_at: 1, groups: [], accounts: [] },
    },
  })
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
  fireEvent.change(
    screen.getByLabelText('Channel groups (comma separated, blank for all)'),
    { target: { value: 'Channel pool' } }
  )
  fireEvent.change(
    screen.getByLabelText('Sub groups (comma separated, blank for all)'),
    { target: { value: 'Sub pool' } }
  )
  expect(
    screen.queryByRole('switch', { name: 'Sub account excerpts' })
  ).not.toBeInTheDocument()
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
    channel_group_filter: 'Channel pool',
    sub_group_filter: 'Sub pool',
    show_sub_details: false,
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
  expect(
    screen.getByLabelText('Channel groups (comma separated, blank for all)')
  ).toHaveValue('Channel pool')
  expect(
    screen.getByLabelText('Sub groups (comma separated, blank for all)')
  ).toHaveValue('Sub pool')
  second.client.clear()
})

test('loads the legacy shared selection into both independent fields and saves an explicit blank independently', async () => {
  mocks.load.mockResolvedValue({
    success: true,
    data: [
      {
        key: STATUS_IMAGE_OPTION,
        value: JSON.stringify({
          ...defaultStatusImageConfig,
          group_filter: 'Legacy pool',
          channel_group_filter: undefined,
          sub_group_filter: undefined,
        }),
      },
    ],
  })
  const { client } = await setup()
  const channel = screen.getByLabelText(
    'Channel groups (comma separated, blank for all)'
  )
  const sub = screen.getByLabelText(
    'Sub groups (comma separated, blank for all)'
  )
  expect(channel).toHaveValue('Legacy pool')
  expect(sub).toHaveValue('Legacy pool')
  expect(mocks.save).not.toHaveBeenCalled()
  fireEvent.change(sub, { target: { value: '' } })
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  expect(channel).toHaveValue('Legacy pool')
  expect(sub).toHaveValue('')
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() => expect(mocks.save).toHaveBeenCalledOnce())
  expect(JSON.parse(mocks.save.mock.calls[0][0].value)).toMatchObject({
    channel_group_filter: 'Legacy pool',
    sub_group_filter: '',
  })
  client.clear()
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
  fireEvent.change(
    screen.getByLabelText('Channel groups (comma separated, blank for all)'),
    { target: { value: 'Pending channel' } }
  )
  fireEvent.change(
    screen.getByLabelText('Sub groups (comma separated, blank for all)'),
    { target: { value: 'Pending sub' } }
  )
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
  expect(
    screen.getByLabelText('Channel groups (comma separated, blank for all)')
  ).toHaveValue('Pending channel')
  expect(
    screen.getByLabelText('Sub groups (comma separated, blank for all)')
  ).toHaveValue('Pending sub')
  mocks.save.mockResolvedValue({ success: false, message: 'write failed' })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await screen.findByText('write failed')
  expect(screen.getByLabelText('Report site name')).toHaveValue('Pending name')
  expect(screen.getByRole('button', { name: 'Discard changes' })).toBeEnabled()
  expect(
    screen.getByLabelText('Channel groups (comma separated, blank for all)')
  ).toHaveValue('Pending channel')
  expect(
    screen.getByLabelText('Sub groups (comma separated, blank for all)')
  ).toHaveValue('Pending sub')
  client.clear()
})

test('shows a live data failure without silently exporting sample data', async () => {
  mocks.live.mockRejectedValue(new Error('offline'))
  const { client } = await setup()
  await screen.findByText('Failed to load channel statistics')
  expect(
    screen.queryByRole('button', { name: 'Download PNG' })
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  expect(screen.getByRole('tab', { name: 'Sub status' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  await screen.findByText('Failed to load Sub account status')
  expect(
    screen.queryByRole('button', { name: 'Download PNG' })
  ).not.toBeInTheDocument()
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

test('requests live Sub data rather than silently substituting example accounts', async () => {
  const { client } = await setup()
  mocks.live.mockClear()
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  await waitFor(() => expect(mocks.live).toHaveBeenCalledWith('sub'))
  client.clear()
})

test('disconnected Sub response offers no image download or sample-account substitution', async () => {
  const { client } = await setup()
  mocks.live.mockResolvedValue({
    success: true,
    data: {
      config: defaultStatusImageConfig,
      sub: {
        status: 'failed',
        checked_at: 1,
        error: 'Connection unavailable',
        groups: [],
        accounts: [],
      },
    },
  })
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  await screen.findByText('Failed to load Sub account status')
  expect(
    screen.queryByRole('button', { name: 'Download PNG' })
  ).not.toBeInTheDocument()
  client.clear()
})

test('preview focus retains edits while updating its accessible state', async () => {
  const { client } = await setup()
  fireEvent.change(screen.getByLabelText('Report site name'), {
    target: { value: 'Pending workspace' },
  })
  const expand = screen.getByRole('button', { name: 'Expand preview' })
  fireEvent.click(expand)
  expect(
    screen.getByRole('button', { name: 'Exit expanded preview' })
  ).toHaveAttribute('aria-pressed', 'true')
  fireEvent.click(screen.getByRole('button', { name: 'Exit expanded preview' }))
  expect(screen.getByLabelText('Report site name')).toHaveValue(
    'Pending workspace'
  )
  expect(
    screen.getByRole('button', { name: 'Expand preview' })
  ).toHaveAttribute('aria-pressed', 'false')
  client.clear()
})

test('saves and reloads exact model selections independently of Sub settings and keeps clearing as none', async () => {
  mocks.live.mockResolvedValue({
    success: true,
    data: {
      config: defaultStatusImageConfig,
      channel: {
        enabled: true,
        mode: 'user',
        checked_at: 1,
        groups: ['A', 'B'].map((group) => ({
          group,
          models: [
            { model: 'gpt-5', cards: [] },
            { model: 'claude-sonnet', cards: [] },
          ],
        })),
      },
      sub: { status: 'connected', checked_at: 1, groups: [], accounts: [] },
    },
  })
  const first = await setup()
  expect(screen.getByRole('button', { name: 'All models' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(mocks.save).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Selected models only' }))
  fireEvent.click(await screen.findByRole('checkbox', { name: 'A / gpt-5' }))
  fireEvent.click(screen.getByRole('checkbox', { name: 'B / claude-sonnet' }))
  expect(screen.getByRole('checkbox', { name: 'B / gpt-5' })).not.toBeChecked()
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() => expect(mocks.save).toHaveBeenCalledOnce())
  const payload = mocks.save.mock.calls[0][0]
  expect(JSON.parse(payload.value)).toMatchObject({
    channel_model_filter: [
      { group: 'A', model: 'gpt-5' },
      { group: 'B', model: 'claude-sonnet' },
    ],
    sub_group_filter: '',
  })
  first.unmount()
  first.client.clear()
  mocks.load.mockResolvedValue({ success: true, data: [payload] })
  const second = await setup()
  expect(screen.getByRole('checkbox', { name: 'A / gpt-5' })).toBeChecked()
  expect(
    await screen.findByRole('checkbox', { name: 'B / claude-sonnet' })
  ).toBeChecked()
  fireEvent.click(screen.getByRole('button', { name: 'Clear model selection' }))
  expect(
    screen.getByRole('button', { name: 'Selected models only' })
  ).toHaveAttribute('aria-pressed', 'true')
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() => expect(mocks.save).toHaveBeenCalledTimes(2))
  expect(
    JSON.parse(mocks.save.mock.calls[1][0].value).channel_model_filter
  ).toEqual([])
  second.client.clear()
})

test('current channel groups update during a Sub preview without discarding unsaved edits or saving automatically', async () => {
  mocks.load.mockResolvedValue({
    success: true,
    data: [
      {
        key: STATUS_IMAGE_OPTION,
        value: JSON.stringify({
          ...defaultStatusImageConfig,
          channel_group_filter: 'Deleted pool',
          channel_model_filter: [{ group: 'Deleted pool', model: 'gpt-5' }],
        }),
      },
    ],
  })
  const response = (groups: string[]) => ({
    success: true,
    data: {
      config: defaultStatusImageConfig,
      channel: {
        enabled: true,
        mode: 'user',
        checked_at: 1,
        groups: groups.map((group) => ({
          group,
          models: [{ model: 'gpt-5', cards: [] }],
        })),
      },
      sub: { status: 'connected', checked_at: 1, groups: [], accounts: [] },
    },
  })
  mocks.live.mockResolvedValue(response(['Current pool', 'Deleted pool']))
  const { client } = await setup()
  expect(
    await screen.findByRole('checkbox', { name: 'Deleted pool / gpt-5' })
  ).toBeChecked()
  fireEvent.change(screen.getByLabelText('Report site name'), {
    target: { value: 'Pending site name' },
  })
  fireEvent.click(screen.getByRole('tab', { name: 'Sub status' }))
  await waitFor(() => expect(mocks.live).toHaveBeenCalledWith('sub'))
  mocks.live.mockResolvedValue(response(['Current pool']))

  await client.refetchQueries({
    queryKey: ['status-image-overview', 'channel'],
  })

  await waitFor(() =>
    expect(
      screen.queryByRole('checkbox', { name: 'Deleted pool / gpt-5' })
    ).not.toBeInTheDocument()
  )
  expect(
    screen.getByRole('checkbox', { name: 'Current pool / gpt-5' })
  ).not.toBeChecked()
  expect(screen.getByLabelText('Report site name')).toHaveValue(
    'Pending site name'
  )
  expect(mocks.save).not.toHaveBeenCalled()
  expect(
    screen.getByRole('button', { name: 'Selected models only' })
  ).toHaveAttribute('aria-pressed', 'true')
  client.clear()
})
