/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  addProfitabilityEntry,
  getProfitabilitySummary,
} from '../api'
import { ProfitabilityCalculator } from '../profitability-calculator'

vi.mock('../api', () => ({
  addProfitabilityEntry: vi.fn(),
  getProfitabilitySummary: vi.fn(),
}))

const report = {
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
  channels: [
    { id: 7, name: 'Sub channel', purchase_rmb: 25, sub_usage_usd: 10, customer_charge_rmb: 30, estimated_profit_rmb: 5, matched_requests: 3, unmatched_requests: 0 },
  ],
  models: [
    { channel_id: 7, name: 'test-model', sub_usage_usd: 10, customer_charge_rmb: 30, allocated_cost_rmb: 25, estimated_profit_rmb: 5 },
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
      customer_usd: 8,
      self_usd: 0,
      profitable: true,
      profit_status: 'profit',
    },
    {
      name: 'low-cost',
      ratio: 2,
      customer_usd: 2,
      self_usd: 0,
      profitable: false,
      profit_status: 'loss',
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
    vi.mocked(getProfitabilitySummary).mockResolvedValue(report)
    vi.mocked(addProfitabilityEntry).mockResolvedValue()
  })

  test('shows channel and model margin when Sub calls are matched', async () => {
    renderCalculator()

    await waitFor(() => {
      expect(screen.getAllByText('¥25.00').length).toBeGreaterThan(0)
      expect(screen.getAllByText('$10.00').length).toBeGreaterThan(0)
      expect(screen.getByText('$3.00')).toBeInTheDocument()
      expect(screen.getAllByText('¥5.00').length).toBeGreaterThan(0)
    })
    expect(screen.getByText('test-model')).toBeInTheDocument()
    expect(screen.getByText(/#42:/)).toBeInTheDocument()
  })

  test('submits one account purchase when the form is submitted with Enter', async () => {
    renderCalculator()

    await screen.findByRole('option', { name: 'Sub channel (#7)' })
    const countInput = screen.getByLabelText('Number of accounts')
    fireEvent.change(screen.getByLabelText('Channel'), { target: { value: '7' } })
    const amountInput = screen.getByLabelText('Total cost (RMB)')
    fireEvent.change(countInput, { target: { value: '3' } })
    fireEvent.change(amountInput, { target: { value: '12.50' } })
    fireEvent.submit(amountInput.closest('form') as HTMLFormElement)

    await waitFor(() => {
      expect(addProfitabilityEntry).toHaveBeenCalledTimes(1)
    })
    expect(vi.mocked(addProfitabilityEntry).mock.calls[0]?.[0]).toEqual({
      entry_type: 'account_purchase',
      amount_usd: 0,
      amount_rmb: 12.5,
      account_count: 3,
      channel_id: 7,
      note: '',
    })
  })

  test('submits one private payment entry when the form is submitted', async () => {
    renderCalculator()

    const amountInput = screen.getByLabelText('Received amount (USD)')
    fireEvent.change(amountInput, { target: { value: '7.25' } })
    fireEvent.submit(amountInput.closest('form') as HTMLFormElement)

    await waitFor(() => {
      expect(addProfitabilityEntry).toHaveBeenCalledTimes(1)
    })
    expect(vi.mocked(addProfitabilityEntry).mock.calls[0]?.[0]).toEqual({
      entry_type: 'manual_income',
      amount_usd: 7.25,
      account_count: undefined,
      channel_id: undefined,
      note: '',
    })
  })

  test('renders an empty report when list fields are null', async () => {
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      groups: null,
      usage: { ...report.usage, users: null, groups: null },
    } as unknown as typeof report)

    renderCalculator()

    await waitFor(() => {
      expect(screen.getByText('No channels in this period')).toBeInTheDocument()
    })
  })

  test('cash mode excludes unspent receipts from recognized calls', async () => {
    renderCalculator()
    fireEvent.click(screen.getByRole('button', { name: 'Cash received' }))
    await waitFor(() => expect(screen.getByText('¥20.00')).toBeInTheDocument())
    expect(screen.getByText('-¥5.00')).toBeInTheDocument()
    expect(screen.queryByText('Model profitability')).not.toBeInTheDocument()
  })

  test('shows incomplete channel profit when Sub usage has no matching New API call', async () => {
    vi.mocked(getProfitabilitySummary).mockResolvedValue({
      ...report,
      unmatched_sub_requests: 1,
      channels: [{ ...report.channels[0], estimated_profit_rmb: null }],
      estimated_profit_rmb: null,
    })
    renderCalculator()

    expect(await screen.findByText('Unmatched calls prevent a complete estimate')).toBeInTheDocument()
    expect(screen.queryByText('Unmatched calls: 0')).not.toBeInTheDocument()
  })
})
