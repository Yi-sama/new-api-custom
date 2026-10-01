/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import {
  addProfitabilityEntry,
  getProfitabilitySummary,
  type ProfitabilityPoolSummary,
  type ProfitabilityReport,
} from '../api'
import { ProfitabilityCalculator } from '../profitability-calculator'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    addProfitabilityEntry: vi.fn(),
    getProfitabilitySummary: vi.fn(),
  }
})

const availableChannels = [
  { id: 7, name: 'Sub channel', status: 1 },
  { id: 12, name: 'Disabled history', status: 2 },
]

const pool: ProfitabilityPoolSummary = {
  all_calls: {
    charge_usd: 11,
    sub_usage_usd: 10,
    effective_ratio: 1.1,
    status: 'complete',
  },
  customer_calls: {
    charge_usd: 8,
    sub_usage_usd: 7,
    effective_ratio: 8 / 7,
    status: 'complete',
  },
  admin_charge_usd: 3,
  matched_admin_sub_usage_usd: 3,
  outside_selected_sub_usage_usd: 0,
  profit_rmb: 5,
  profit_status: 'complete',
}

const report = {
  pool,
  revenue_usd: 25,
  topup_revenue_usd: 20,
  manual_income_usd: 5,
  admin_adjustment_usd: 0,
  account_cost_rmb: 25,
  sub_usage_usd: 10,
  cost_rate_rmb_per_usd: 2.5,
  account_count: 2,
  account_purchase_count: 2,
  sub_usage_status: 'connected',
  available_channels: availableChannels,
  scope: 'selected',
  selected_channel_ids: [7],
  profit_status: 'complete',
  matched_requests: 3,
  unmatched_requests: 0,
  unmatched_sub_usage_usd: 0,
  channels: [
    {
      id: 7,
      name: 'Sub channel',
      status: 1,
      profit_status: 'complete',
      purchase_recorded: true,
      purchase_rmb: 25,
      sub_usage_usd: 10,
      customer_charge_rmb: 30,
      estimated_profit_rmb: 5,
      matched_requests: 3,
      unmatched_requests: 0,
    },
  ],
  models: [
    {
      channel_id: 7,
      name: 'test-model',
      sub_usage_usd: 10,
      customer_charge_rmb: 30,
      allocated_cost_rmb: 25,
      estimated_profit_rmb: 5,
      profit_status: 'complete',
    },
  ],
  accounts: [{ channel_id: 7, account_id: 42, sub_usage_usd: 10 }],
  unmatched_sub_requests: 0,
  cash_received_rmb: 25,
  cash_recognized_rmb: 20,
  cash_profit_rmb: -5,
  call_charge_rmb: 30,
  estimated_profit_rmb: 5,
  groups: [
    {
      name: 'default',
      ratio: 3,
      configured_ratio: 1.25,
      customer_usd: 8,
      self_usd: 0,
      profitable: true,
      profit_status: 'complete',
      estimated_profit_rmb: 4,
      all_calls: {
        charge_usd: 5.5,
        sub_usage_usd: 5,
        effective_ratio: 1.1,
        status: 'complete',
        break_even_ratio: 0.75,
        break_even_status: 'complete',
      },
      customer_calls: {
        charge_usd: 3.6,
        sub_usage_usd: 4,
        effective_ratio: 0.9,
        status: 'complete',
        break_even_ratio: 0.8,
        break_even_status: 'complete',
      },
    },
    {
      name: 'premium-enterprise-group-with-a-very-long-name',
      ratio: 4,
      configured_ratio: 2.4,
      customer_usd: 4,
      self_usd: 3,
      profitable: true,
      profit_status: 'complete',
      estimated_profit_rmb: 1,
      all_calls: {
        charge_usd: 5.5,
        sub_usage_usd: 2.5,
        effective_ratio: 2.2,
        status: 'complete',
        break_even_ratio: 1.5,
        break_even_status: 'complete',
      },
      customer_calls: {
        charge_usd: 4.5,
        sub_usage_usd: 2.5,
        effective_ratio: 1.8,
        status: 'complete',
        break_even_ratio: 1.6,
        break_even_status: 'complete',
      },
    },
  ],
  usage: {
    quota: 1_500_000,
    amount_usd: 3,
    customer_usd: 8,
    self_usd: 3,
    request_count: 4,
    token_count: 1200,
    users: [],
    groups: [],
  },
} satisfies ProfitabilityReport

const emptySelectionReport: ProfitabilityReport = {
  ...report,
  account_cost_rmb: 0,
  sub_usage_usd: 0,
  account_count: 0,
  account_purchase_count: 0,
  scope: 'selected',
  selected_channel_ids: [],
  profit_status: 'empty_selection',
  matched_requests: 0,
  channels: [],
  models: [],
  accounts: [],
  call_charge_rmb: 0,
  estimated_profit_rmb: null,
  groups: [],
  usage: { ...report.usage, customer_usd: 0, self_usd: 0 },
}

function setStoredSelection(channelIds: number[], userId = 1) {
  localStorage.setItem(
    `profitability:channel-selection:${userId}`,
    JSON.stringify(channelIds)
  )
}

function renderCalculator() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ProfitabilityCalculator />
    </QueryClientProvider>
  )
}

describe('ProfitabilityCalculator', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'admin-one', role: 100 })
    vi.mocked(getProfitabilitySummary).mockImplementation(
      async (_range, revenueBasis, channelIds) => {
        if (revenueBasis === 'cash') {
          return {
            ...report,
            scope: 'all',
            selected_channel_ids: [],
            pool: {
              ...pool,
              all_calls: {
                ...pool.all_calls,
                effective_ratio: null,
                status: 'cash_scope',
              },
              customer_calls: {
                ...pool.customer_calls,
                effective_ratio: null,
                status: 'cash_scope',
              },
            },
          }
        }
        return channelIds?.length ? report : emptySelectionReport
      }
    )
    vi.mocked(addProfitabilityEntry).mockResolvedValue()
  })

  test('starts with an explicit empty channel request and selecting or clearing updates it', async () => {
    renderCalculator()

    await waitFor(() =>
      expect(getProfitabilitySummary).toHaveBeenCalledWith(
        expect.any(Object),
        'calls',
        []
      )
    )
    expect(
      await screen.findByText('No statistical channels selected')
    ).toBeInTheDocument()

    fireEvent.click(
      screen.getByRole('button', { name: 'Statistical channels' })
    )
    const channelDialog = await screen.findByRole('dialog')
    fireEvent.click(
      within(channelDialog).getByRole('option', { name: /Sub channel \(#7\)/ })
    )

    await waitFor(() =>
      expect(getProfitabilitySummary).toHaveBeenCalledWith(
        expect.any(Object),
        'calls',
        [7]
      )
    )
    fireEvent.click(
      screen.getByRole('option', { name: 'Clear channel selection' })
    )
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        []
      )
    )
  })

  test('keeps the channel catalog available while a new scope is loading', async () => {
    let resolveSelectedReport:
      | ((value: ProfitabilityReport) => void)
      | undefined
    vi.mocked(getProfitabilitySummary).mockImplementation(
      async (_range, _revenueBasis, channelIds) => {
        if (!channelIds?.length) return emptySelectionReport
        return new Promise<ProfitabilityReport>((resolve) => {
          resolveSelectedReport = resolve
        })
      }
    )
    renderCalculator()

    await screen.findByText('No statistical channels selected')
    fireEvent.click(
      screen.getByRole('button', { name: 'Statistical channels' })
    )
    const channelDialog = await screen.findByRole('dialog')
    fireEvent.click(
      within(channelDialog).getByRole('option', { name: /Sub channel \(#7\)/ })
    )
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        [7]
      )
    )

    fireEvent.click(
      within(channelDialog).getByRole('option', {
        name: /Disabled history \(#12\)/,
      })
    )
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        [7, 12]
      )
    )
    expect(
      within(channelDialog).getByRole('option', {
        name: /Disabled history \(#12\)/,
      })
    ).toBeInTheDocument()

    resolveSelectedReport?.(report)
  })

  test('reloads the saved IDs per user without selecting newly available channels', async () => {
    setStoredSelection([7])
    const view = renderCalculator()

    await waitFor(() =>
      expect(getProfitabilitySummary).toHaveBeenCalledWith(
        expect.any(Object),
        'calls',
        [7]
      )
    )
    expect(
      screen.getByRole('button', { name: 'Statistical channels' })
    ).toHaveTextContent('1 channels selected')

    view.unmount()
    renderCalculator()
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        [7]
      )
    )

    act(() => {
      useAuthStore
        .getState()
        .auth.setUser({ id: 2, username: 'admin-two', role: 100 })
    })
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        []
      )
    )
    expect(
      screen.getByRole('button', { name: 'Statistical channels' })
    ).toHaveTextContent('Choose channels')
    expect(localStorage.getItem('profitability:channel-selection:2')).toBeNull()
  })

  test('requests cash globally and restores the saved call selection', async () => {
    setStoredSelection([7])
    renderCalculator()

    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        [7]
      )
    )
    fireEvent.click(screen.getByRole('button', { name: 'Cash received' }))
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)).toEqual([
        expect.any(Object),
        'cash',
        undefined,
      ])
    )
    expect(
      screen.getByRole('button', { name: 'Statistical channels' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Statistical channels' })
    ).toHaveTextContent('All channels (cash)')

    fireEvent.click(screen.getByRole('button', { name: 'Call charges' }))
    await waitFor(() =>
      expect(vi.mocked(getProfitabilitySummary).mock.calls.at(-1)?.[2]).toEqual(
        [7]
      )
    )
  })

  test('keeps full Sub usage visible and hides every group break-even value in cash view', async () => {
    setStoredSelection([7])
    renderCalculator()

    fireEvent.click(
      await screen.findByRole('button', { name: 'Cash received' })
    )

    expect(
      (await screen.findAllByText('Cash view has no pool ratio')).length
    ).toBeGreaterThan(0)
    expect(
      within(
        screen.getByRole('region', { name: 'Full pool Sub usage' })
      ).getByText('$10.00')
    ).toBeInTheDocument()
    const table = screen.getByRole('region', { name: 'Group ratio table' })
    expect(within(table).getByText('1.250')).toBeInTheDocument()
    expect(within(table).getByText('2.400')).toBeInTheDocument()
    expect(within(table).queryByText('0.750')).not.toBeInTheDocument()
    expect(within(table).queryByText('1.500')).not.toBeInTheDocument()
    expect(
      within(table).getAllByText('Cash view has no pool ratio')
    ).toHaveLength(2)
    expect(screen.getByLabelText('Cash received (RMB)')).toHaveTextContent(
      '¥25.00'
    )
    expect(
      screen.getByRole('region', {
        name: 'Customer paid usage revenue (RMB)',
      })
    ).toHaveTextContent('¥20.00')
  })

  test('defaults to the full pool and switches headline and per-group metrics to customers only', async () => {
    setStoredSelection([7])
    renderCalculator()

    const allCallsButton = await screen.findByRole('button', {
      name: 'Include administrators',
    })
    expect(allCallsButton).toHaveAttribute('aria-pressed', 'true')
    expect(
      within(
        screen.getByRole('region', { name: 'Total charge (USD)' })
      ).getByText('$11.00')
    ).toBeInTheDocument()
    expect(
      within(
        screen.getByRole('region', { name: 'Full pool Sub usage' })
      ).getByText('$10.00')
    ).toBeInTheDocument()
    const table = screen.getByRole('region', { name: 'Group ratio table' })
    expect(within(table).getByText('1.100')).toBeInTheDocument()
    expect(within(table).getByText('2.200')).toBeInTheDocument()
    expect(within(table).getByText('0.750')).toBeInTheDocument()
    expect(within(table).getByText('1.500')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Customers only' }))

    expect(
      within(
        screen.getByRole('region', { name: 'Total charge (USD)' })
      ).getByText('$8.00')
    ).toBeInTheDocument()
    expect(
      within(
        screen.getByRole('region', { name: 'Customer Sub usage' })
      ).getByText('$7.00')
    ).toBeInTheDocument()
    expect(within(table).getByText('0.900')).toBeInTheDocument()
    expect(within(table).getByText('1.800')).toBeInTheDocument()
    expect(within(table).getByText('0.800')).toBeInTheDocument()
    expect(within(table).getByText('1.600')).toBeInTheDocument()
  })

  test('keeps small per-group ratios visible without rounding them to zero', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      groups: [
        {
          ...report.groups[0],
          all_calls: {
            ...report.groups[0].all_calls,
            effective_ratio: 0.0375,
          },
          customer_calls: {
            ...report.groups[0].customer_calls,
            effective_ratio: 0.0004,
          },
        },
      ],
    })
    renderCalculator()

    const table = await screen.findByRole('region', {
      name: 'Group ratio table',
    })
    expect(within(table).getByText('0.0375')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Customers only' }))
    expect(within(table).getByText('0.0004')).toBeInTheDocument()
  })

  test('shows each group configured, effective, and break-even ratio independently', async () => {
    setStoredSelection([7])
    renderCalculator()

    const table = await screen.findByRole('region', {
      name: 'Group ratio table',
    })
    const defaultRow = within(table).getByRole('row', { name: /default/ })
    expect(defaultRow).toHaveTextContent('1.250')
    expect(defaultRow).toHaveTextContent('1.100')
    expect(defaultRow).toHaveTextContent('0.750')
    const premiumRow = within(table).getByRole('row', {
      name: /premium-enterprise-group-with-a-very-long-name/,
    })
    expect(premiumRow).toHaveTextContent('2.400')
    expect(premiumRow).toHaveTextContent('2.200')
    expect(premiumRow).toHaveTextContent('1.500')
  })

  test('places the configured ratio immediately before status with aligned row values', async () => {
    setStoredSelection([7])
    renderCalculator()

    const table = await screen.findByRole('region', {
      name: 'Group ratio table',
    })
    expect(
      within(table)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent)
    ).toEqual([
      'Group',
      'Charge (USD)',
      'Sub usage (USD)',
      'Actual charge / Sub ratio',
      'Estimated group break-even ratio',
      'Current configured ratio',
      'Status',
    ])
    const row = within(table).getByRole('row', { name: /default/ })
    expect(
      within(row)
        .getAllByRole('cell')
        .map((cell) => cell.textContent)
    ).toEqual(['$5.50', '$5.00', '1.100', '0.750', '1.250', 'Complete'])
  })

  test('does not use legacy ratio fields when the backend omits new group metrics', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      groups: [
        {
          name: 'legacy-group',
          ratio: 9,
          customer_usd: 8,
          self_usd: 1,
          profitable: true,
          profit_status: 'complete',
          break_even_ratio: 7,
        },
      ],
    })
    renderCalculator()

    const row = within(
      await screen.findByRole('region', { name: 'Group ratio table' })
    ).getByRole('row', { name: /legacy-group/ })
    expect(within(row).getAllByText('Unknown')).toHaveLength(6)
    expect(row).not.toHaveTextContent('9.000')
    expect(row).not.toHaveTextContent('7.000')
  })

  test('keeps zero rates and long content inside a keyboard-focusable local scroller', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      groups: [
        {
          ...report.groups[1],
          configured_ratio: 0,
          all_calls: {
            ...report.groups[1].all_calls,
            charge_usd: 1234567890123,
          },
        },
      ],
    })
    renderCalculator()

    const table = await screen.findByRole('region', {
      name: 'Group ratio table',
    })
    expect(within(table).getByText('0.000')).toBeInTheDocument()
    expect(table).toHaveClass('max-w-full', 'overflow-x-auto')
    expect(table).toHaveAttribute('tabindex', '0')
    table.focus()
    expect(table).toHaveFocus()
    expect(
      within(table).getByText('premium-enterprise-group-with-a-very-long-name')
    ).toHaveClass('break-words', 'whitespace-normal')
    expect(within(table).getByText('$1,234,567,890,123.00')).toHaveClass(
      'break-all'
    )
  })

  test.each([
    ['missing_ratio', 'Call-time ratio is missing'],
    ['ratio_override', 'Uses a channel or user-specific ratio'],
  ])(
    'explains %s without displaying a stale break-even value',
    async (status, label) => {
      setStoredSelection([7])
      vi.mocked(getProfitabilitySummary).mockResolvedValue({
        ...report,
        groups: [
          {
            ...report.groups[0],
            all_calls: {
              ...report.groups[0].all_calls,
              break_even_status: status,
            },
          },
        ],
      })
      renderCalculator()

      const table = await screen.findByRole('region', {
        name: 'Group ratio table',
      })
      expect(within(table).getByText(label)).toBeInTheDocument()
      expect(within(table).getByText('1.250')).toBeInTheDocument()
      expect(within(table).getByText('Unknown')).toBeInTheDocument()
      expect(within(table).queryByText('0.750')).not.toBeInTheDocument()
    }
  )

  test('uses pool all-call usage instead of the legacy matched usage', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      sub_usage_usd: 99,
    })
    renderCalculator()

    const usageMetric = await screen.findByRole('region', {
      name: 'Full pool Sub usage',
    })
    expect(within(usageMetric).getByText('$10.00')).toBeInTheDocument()
    expect(screen.queryByText('$99.00')).not.toBeInTheDocument()
  })

  test('shows data not ready when an older backend omits pool', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      pool: undefined,
      sub_usage_usd: 99,
    })
    renderCalculator()

    expect(
      (await screen.findAllByText('Data not ready')).length
    ).toBeGreaterThan(0)
    expect(
      screen.getByRole('region', { name: 'Full pool Sub usage' })
    ).toHaveTextContent('-')
    expect(screen.queryByText('$99.00')).not.toBeInTheDocument()
  })

  test('keeps the real operating profit unchanged when the call scope changes', async () => {
    setStoredSelection([7])
    renderCalculator()

    const operatingLedger = (
      await screen.findByText('Operating ledger')
    ).closest('section')
    expect(operatingLedger).not.toBeNull()
    expect(
      within(operatingLedger as HTMLElement).getByText('¥5.00')
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Customers only' }))
    expect(
      within(operatingLedger as HTMLElement).getByText('¥5.00')
    ).toBeInTheDocument()
  })

  test('shows scope mismatch as an unknown ratio without hiding known pool usage', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      pool: {
        ...pool,
        all_calls: {
          ...pool.all_calls,
          effective_ratio: null,
          status: 'scope_mismatch',
        },
      },
      groups: [
        {
          ...report.groups[0],
          all_calls: {
            ...report.groups[0].all_calls,
            effective_ratio: null,
            status: 'scope_mismatch',
            break_even_ratio: null,
            break_even_status: 'scope_mismatch',
          },
        },
      ],
    })
    renderCalculator()

    expect(
      (
        await screen.findAllByText(
          'Selected scope does not cover the full pool'
        )
      ).length
    ).toBeGreaterThan(0)
    expect(
      within(
        screen.getByRole('region', { name: 'Full pool Sub usage' })
      ).getByText('$10.00')
    ).toBeInTheDocument()
    const groupRow = within(
      screen.getByRole('region', { name: 'Group ratio table' })
    ).getByRole('row', { name: /default/ })
    expect(within(groupRow).getAllByText('Unknown')).toHaveLength(2)
    expect(groupRow).toHaveTextContent(
      'Selected scope does not cover the full pool'
    )
  })

  test('renders the group ratio table without legacy model allocation sections', async () => {
    setStoredSelection([7])
    renderCalculator()

    await screen.findByText('Account pool ledger')
    expect(
      await screen.findByRole('region', { name: 'Group ratio table' })
    ).toBeInTheDocument()
    expect(screen.queryByText('Model profitability')).not.toBeInTheDocument()
    expect(screen.queryByText('Group pricing')).not.toBeInTheDocument()
    expect(screen.queryByText('Allocated cost (RMB)')).not.toBeInTheDocument()
  })

  test.each([
    ['not_configured', 'not_configured'],
    ['error', 'sync_error'],
  ] as const)(
    'shows a dash instead of zero when Sub status is %s',
    async (subStatus, profitStatus) => {
      setStoredSelection([7])
      vi.mocked(getProfitabilitySummary).mockResolvedValue({
        ...report,
        sub_usage_status: subStatus,
        sub_usage_usd: 0,
        profit_status: profitStatus,
        pool: {
          ...pool,
          all_calls: {
            ...pool.all_calls,
            sub_usage_usd: null,
            effective_ratio: null,
            status: profitStatus,
          },
          customer_calls: {
            ...pool.customer_calls,
            sub_usage_usd: null,
            effective_ratio: null,
            status: profitStatus,
          },
        },
        estimated_profit_rmb: null,
        channels: [
          {
            ...report.channels[0],
            sub_usage_usd: 0,
            profit_status: profitStatus,
            estimated_profit_rmb: null,
          },
        ],
      })
      renderCalculator()

      const metric = await screen.findByRole('region', {
        name: 'Full pool Sub usage',
      })
      expect(within(metric).getByText('-')).toBeInTheDocument()
      expect(within(metric).queryByText('$0.00')).not.toBeInTheDocument()
      const adminUsage = screen
        .getByText('Matched administrator Sub usage (USD)')
        .closest('div')
      expect(adminUsage).not.toBeNull()
      expect(
        within(adminUsage as HTMLElement).getByText('-')
      ).toBeInTheDocument()
      const outsideUsage = screen
        .getByText('Outside selected channels Sub usage (USD)')
        .closest('div')
      expect(outsideUsage).not.toBeNull()
      expect(
        within(outsideUsage as HTMLElement).getByText('-')
      ).toBeInTheDocument()
    }
  )

  test('labels an unverified all-call ratio without claiming complete attribution', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      profit_status: 'unmatched',
      matched_requests: 3,
      unmatched_requests: 2,
      estimated_profit_rmb: null,
      pool: {
        ...pool,
        all_calls: {
          ...pool.all_calls,
          status: 'unverified_scope',
        },
        customer_calls: {
          ...pool.customer_calls,
          sub_usage_usd: null,
          effective_ratio: null,
          status: 'unmatched',
        },
      },
      groups: [
        {
          ...report.groups[0],
          all_calls: {
            ...report.groups[0].all_calls,
            status: 'unverified_scope',
            break_even_status: 'unverified_scope',
          },
          customer_calls: {
            ...report.groups[0].customer_calls,
            sub_usage_usd: null,
            effective_ratio: null,
            status: 'unmatched',
            break_even_ratio: null,
            break_even_status: 'unmatched',
          },
        },
      ],
      channels: [
        {
          ...report.channels[0],
          profit_status: 'unmatched',
          unmatched_requests: 2,
          estimated_profit_rmb: null,
        },
      ],
    })
    renderCalculator()

    expect(
      (await screen.findAllByText('Totals available; attribution incomplete'))
        .length
    ).toBeGreaterThan(0)
    const groupTable = screen.getByRole('region', {
      name: 'Group ratio table',
    })
    expect(within(groupTable).getByText('1.100')).toBeInTheDocument()
    expect(within(groupTable).queryByText('0.750')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Customers only' }))
    const groupRow = within(groupTable).getByRole('row', { name: /default/ })
    expect(within(groupRow).getAllByText('Unknown')).toHaveLength(3)
    expect(groupRow).toHaveTextContent(
      'Customer Sub usage cannot be determined'
    )
  })

  test('does not present a missing purchase as free when unmatched is the primary status', async () => {
    setStoredSelection([7])
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      account_cost_rmb: 0,
      account_purchase_count: 0,
      profit_status: 'unmatched',
      estimated_profit_rmb: null,
      pool: {
        ...pool,
        profit_rmb: null,
        profit_status: 'missing_purchase',
      },
      channels: [
        {
          ...report.channels[0],
          profit_status: 'unmatched',
          purchase_recorded: false,
          purchase_rmb: 0,
          estimated_profit_rmb: null,
        },
      ],
    })
    renderCalculator()

    expect(
      await screen.findByText('Some selected channels have no purchase record')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('region', { name: 'Account purchase cost (RMB)' })
    ).toHaveTextContent('-')
    const row = screen.getByRole('row', { name: /Sub channel \(#7\)/ })
    expect(within(row).getAllByText('-').length).toBeGreaterThan(0)
    expect(within(row).queryByText('¥0.00')).not.toBeInTheDocument()
  })

  test('offers only enabled purchase channels while retaining disabled statistical history', async () => {
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...emptySelectionReport,
      available_channels: [
        ...availableChannels,
        { id: 13, name: 'Auto-disabled channel', status: 3 },
        { id: 14, name: 'Deleted channel', status: 0 },
      ],
    })
    renderCalculator()

    const purchaseChannel = await screen.findByRole('combobox', {
      name: 'Channel',
    })
    expect(
      within(purchaseChannel)
        .getAllByRole('option')
        .map((option) => option.textContent)
    ).toEqual(['Select a channel', 'Sub channel (#7)'])

    fireEvent.click(
      screen.getByRole('button', { name: 'Statistical channels' })
    )
    const channelDialog = await screen.findByRole('dialog')
    expect(
      within(channelDialog).getByRole('option', {
        name: /Disabled history \(#12\)/,
      })
    ).toBeInTheDocument()
  })

  test('shows only the purchase placeholder when no channels are enabled', async () => {
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...emptySelectionReport,
      available_channels: [availableChannels[1]],
    })
    renderCalculator()

    const purchaseChannel = await screen.findByRole('combobox', {
      name: 'Channel',
    })
    expect(within(purchaseChannel).getAllByRole('option')).toHaveLength(1)
    expect(
      within(purchaseChannel).getByRole('option', { name: 'Select a channel' })
    ).toBeInTheDocument()
  })

  test('empty selection keeps the channel picker and purchase form available', async () => {
    renderCalculator()

    expect(
      await screen.findByText('No statistical channels selected')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Statistical channels' })
    ).toBeEnabled()
    expect(
      screen.getByRole('option', { name: 'Sub channel (#7)' })
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Number of accounts')).toBeInTheDocument()
    expect(screen.queryByText('Channel profitability')).not.toBeInTheDocument()
  })

  test('uses enabled available channels for purchases instead of the statistical selection', async () => {
    renderCalculator()

    await screen.findByRole('option', {
      name: 'Sub channel (#7)',
    })
    fireEvent.change(screen.getByLabelText('Channel'), {
      target: { value: '7' },
    })
    fireEvent.change(screen.getByLabelText('Number of accounts'), {
      target: { value: '3' },
    })
    const amountInput = screen.getByLabelText('Total cost (RMB)')
    fireEvent.change(amountInput, { target: { value: '12.50' } })
    fireEvent.submit(amountInput.closest('form') as HTMLFormElement)

    await waitFor(() => expect(addProfitabilityEntry).toHaveBeenCalledTimes(1))
    expect(vi.mocked(addProfitabilityEntry).mock.calls[0]?.[0]).toMatchObject({
      entry_type: 'account_purchase',
      amount_rmb: 12.5,
      account_count: 3,
      channel_id: 7,
    })
  })
})
