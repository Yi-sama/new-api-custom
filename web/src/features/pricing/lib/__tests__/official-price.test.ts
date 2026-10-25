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
import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import { getOfficialPriceComparison } from '../official-price'

const grok: PricingModel = {
  id: 1,
  model_name: 'grok-4.7',
  quota_type: 0,
  model_ratio: 0.35,
  completion_ratio: 3,
  cache_ratio: 0.25,
  enable_groups: ['Grok'],
  group_ratio: { Grok: 1 },
}

describe('Official price comparison', () => {
  it('normalizes USD official references against CNY-denominated site credits', () => {
    const value = getOfficialPriceComparison(grok)
    expect(value?.uniformRatio).toBeCloseTo(0.05)
    expect(value?.rows.map((row) => row.officialCny)).toEqual([14, 42, 3.5])
  })

  it('keeps Composer input, output and cached comparisons separate', () => {
    const value = getOfficialPriceComparison({
      ...grok,
      model_name: 'composer-2.5',
      cache_ratio: 0.15,
    })
    expect(value?.uniformRatio).toBeUndefined()
    expect(value?.rows.map((row) => Number(row.ratio.toFixed(6)))).toEqual([
      0.2, 0.12, 0.075,
    ])
  })

  it('uses the selected group and preserves the best free-group zero', () => {
    const value = {
      ...grok,
      enable_groups: ['free', 'Grok'],
      group_ratio: { free: 0, Grok: 1.2 },
    }
    expect(getOfficialPriceComparison(value)?.uniformRatio).toBe(0)
    expect(getOfficialPriceComparison(value, 'Grok')?.uniformRatio).toBeCloseTo(
      0.06
    )
  })

  it('does not infer official references for aliases or excluded groups', () => {
    expect(
      getOfficialPriceComparison({ ...grok, model_name: 'grok-latest' })
    ).toBeNull()
    expect(
      getOfficialPriceComparison({ ...grok, enable_groups: ['满血Claude'] })
    ).toBeNull()
  })

  it('hides expired promotional references and invalid numeric prices', () => {
    expect(
      getOfficialPriceComparison(
        {
          ...grok,
          model_name: 'gemini-4-argon',
        },
        undefined,
        1,
        '2026-12-01'
      )
    ).toBeNull()
    expect(
      getOfficialPriceComparison({
        ...grok,
        model_ratio: Number.NaN,
      })
    ).toBeNull()
  })

  it('uses CNY references without multiplying the exchange rate again', () => {
    const value = getOfficialPriceComparison({
      ...grok,
      model_name: 'minimax-m3',
      enable_groups: ['各种国模'],
      group_ratio: { 各种国模: 1 },
      model_ratio: 1.1025,
      completion_ratio: 4,
      cache_ratio: 0.2,
    })
    expect(value?.uniformRatio).toBeCloseTo(1.05)
    expect(value?.rows[0].officialCny).toBe(2.1)
  })

  it('does not claim a uniform rate when the official cache price is unknown', () => {
    const value = getOfficialPriceComparison({
      ...grok,
      model_name: 'qwen3.8-max',
      enable_groups: ['各种国模'],
      group_ratio: { 各种国模: 1 },
      model_ratio: 6.3,
      cache_ratio: 0.125,
    })
    expect(value?.uniformRatio).toBeUndefined()
    expect(value?.pendingCache).toBe(true)
    expect(value?.rows.map((row) => row.type)).toEqual(['input', 'output'])
  })
})
