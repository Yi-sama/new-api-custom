/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getProfitabilitySummary } from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn() },
}))

const range = { startTimestamp: 10, endTimestamp: 20 }

describe('getProfitabilitySummary', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
  })

  test('sends explicit empty call scope and sorted selected IDs', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: { profit_status: 'empty_selection' } },
    })

    await getProfitabilitySummary(range, 'calls', [])
    expect(api.get).toHaveBeenLastCalledWith('/api/profitability/summary', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        revenue_basis: 'calls',
        channel_ids: '',
      },
    })

    await getProfitabilitySummary(range, 'calls', [12, 7, 12])
    expect(api.get).toHaveBeenLastCalledWith('/api/profitability/summary', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        revenue_basis: 'calls',
        channel_ids: '7,12',
      },
    })
  })

  test('forces cash to the global scope', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: { scope: 'all' } },
    })

    await getProfitabilitySummary(range, 'cash', [7])

    expect(api.get).toHaveBeenCalledWith('/api/profitability/summary', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        revenue_basis: 'cash',
        channel_ids: undefined,
      },
    })
  })

  test('rejects success false instead of returning an empty report', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: false, message: 'scope failed' },
    })

    await expect(getProfitabilitySummary(range, 'calls', [])).rejects.toThrow(
      'scope failed'
    )
  })
})
