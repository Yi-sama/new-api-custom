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
import type { PricingModel } from '../types'
import { getDisplayGroupRatio } from './model-helpers'

type OfficialPrice = {
  currency: 'USD' | 'CNY'
  input: number
  output: number
  cache?: number
  source: string
  validUntil?: string
}

const xai = 'https://docs.x.ai/developers/pricing'
const google = 'https://ai.google.dev/gemini-api/docs/pricing'
const deepseek = 'https://api-docs.deepseek.com/zh-cn/quick_start/pricing/'
const minimax = 'https://platform.minimax.cn/docs/pricing/overview'
const doubao = 'https://www.volcengine.com/docs/82379/1544106'
const kimi = 'https://platform.kimi.com/docs/pricing/chat'
const qwen = 'https://help.aliyun.com/zh/model-studio/model-pricing'

// Standard text references checked on 2026-10-19. Aliases are deliberately
// not inferred: matching an upstream name does not establish its model route.
export const OFFICIAL_PRICES: Readonly<Record<string, OfficialPrice>> = {
  'grok-4.5': { currency: 'USD', input: 2, output: 6, cache: 0.3, source: xai },
  'grok-4.6': { currency: 'USD', input: 2, output: 6, cache: 0.5, source: xai },
  'grok-4.7': { currency: 'USD', input: 2, output: 6, cache: 0.5, source: xai },
  'composer-2.5': {
    currency: 'USD',
    input: 0.5,
    output: 2.5,
    cache: 0.2,
    source: 'https://cursor.com/docs/models-and-pricing',
  },
  'gemini-3.1-flash-lite': {
    currency: 'USD',
    input: 0.25,
    output: 1.5,
    cache: 0.025,
    source: google,
  },
  'gemini-3.8-flash': {
    currency: 'USD',
    input: 0.75,
    output: 3.75,
    cache: 0.075,
    source: google,
    validUntil: '2026-12-31',
  },
  'gemini-3.6-flash': {
    currency: 'USD',
    input: 0.75,
    output: 3.75,
    cache: 0.075,
    source: google,
    validUntil: '2026-12-31',
  },
  'gemini-3-flash-preview': {
    currency: 'USD',
    input: 0.5,
    output: 3,
    cache: 0.05,
    source: google,
  },
  'gemini-4-argon': {
    currency: 'USD',
    input: 2,
    output: 10,
    cache: 0.1,
    source: google,
    validUntil: '2026-11-30',
  },
  'deepseek-flash': {
    currency: 'CNY',
    input: 2,
    output: 8,
    cache: 0.04,
    source: deepseek,
  },
  'deepseek-v4-pro': {
    currency: 'CNY',
    input: 9,
    output: 27,
    cache: 0.3,
    source: deepseek,
  },
  'doubao-seed-2.0-pro': {
    currency: 'CNY',
    input: 3.2,
    output: 16,
    cache: 0.64,
    source: doubao,
  },
  'doubao-seed-2.0-code': {
    currency: 'CNY',
    input: 3.2,
    output: 16,
    cache: 0.64,
    source: doubao,
  },
  'doubao-seed-2.0-mini': {
    currency: 'CNY',
    input: 0.2,
    output: 2,
    cache: 0.04,
    source: doubao,
  },
  'doubao-seed-2.1-pro': {
    currency: 'CNY',
    input: 6,
    output: 30,
    cache: 1.2,
    source: doubao,
  },
  'doubao-seed-2.1-lite': {
    currency: 'CNY',
    input: 0.8,
    output: 2.7,
    cache: 0.16,
    source: doubao,
  },
  'minimax-m3': {
    currency: 'CNY',
    input: 2.1,
    output: 8.4,
    cache: 0.42,
    source: minimax,
  },
  'kimi-k3': {
    currency: 'CNY',
    input: 20,
    output: 100,
    cache: 2,
    source: kimi,
  },
  'kimi-k2.7-code': {
    currency: 'CNY',
    input: 6.5,
    output: 27,
    cache: 1.3,
    source: kimi,
  },
  'kimi-k2.6': {
    currency: 'CNY',
    input: 6.5,
    output: 27,
    cache: 1.1,
    source: kimi,
  },
  // Qwen 3.8 cache rates require a console quote; do not infer them from HAR.
  'qwen3.8-max': {
    currency: 'CNY',
    input: 12,
    output: 36,
    source: qwen,
  },
  'qwen3.8-flash': {
    currency: 'CNY',
    input: 0.8,
    output: 2.7,
    source: qwen,
  },
  // Standard implicit cache rate: input * 20% (context-cache documentation).
  'qwen3.7-max': {
    currency: 'CNY',
    input: 12,
    output: 36,
    cache: 2.4,
    source: qwen,
  },
  'qwen3.6-plus': {
    currency: 'CNY',
    input: 2,
    output: 12,
    cache: 0.4,
    source: qwen,
  },
}

export type OfficialPriceComparison = {
  source: string
  rows: {
    type: 'input' | 'output' | 'cache'
    officialCny: number
    ratio: number
  }[]
  uniformRatio?: number
  pendingCache: boolean
}

export function isOfficialComparisonModel(model: PricingModel): boolean {
  return (
    model.model_name !== 'Auto' &&
    model.quota_type === 0 &&
    model.enable_groups.some((group) =>
      ['Grok', 'Gemini', '各种国模', 'Composer'].includes(group)
    )
  )
}

export function getOfficialPriceComparison(
  model: PricingModel,
  selectedGroup?: string,
  cnyPerCredit = 1,
  date = new Date().toISOString().slice(0, 10)
): OfficialPriceComparison | null {
  const reference = OFFICIAL_PRICES[model.model_name]
  if (
    !isOfficialComparisonModel(model) ||
    !reference ||
    (reference.validUntil && date > reference.validUntil) ||
    model.billing_expr ||
    !Number.isFinite(cnyPerCredit) ||
    cnyPerCredit <= 0
  ) {
    return null
  }
  // Site credits are purchased in CNY, independently of its display symbol.
  // The user's fixed comparison basis is 7 CNY/USD, not USDExchangeRate=1.
  const referenceFx = reference.currency === 'USD' ? 7 : 1
  const inputCny =
    model.model_ratio *
    2 *
    getDisplayGroupRatio(model, selectedGroup) *
    cnyPerCredit
  const relativeRatios = {
    input: 1,
    output: model.completion_ratio,
    cache: model.cache_ratio,
  }
  const pendingCache = model.cache_ratio != null && reference.cache == null
  const rows: OfficialPriceComparison['rows'] = []
  for (const type of ['input', 'output', 'cache'] as const) {
    const relativeRatio = relativeRatios[type]
    if (relativeRatio == null || reference[type] == null) continue
    if (!Number.isFinite(relativeRatio) || relativeRatio < 0) return null
    const officialCny = reference[type] * referenceFx
    const ratio = (inputCny * relativeRatio) / officialCny
    if (!Number.isFinite(ratio) || ratio < 0) return null
    rows.push({ type, officialCny, ratio })
  }
  if (rows.length === 0) return null
  const uniform = rows.every(
    (row) =>
      Math.abs(row.ratio - rows[0].ratio) <=
      1e-9 * Math.max(1, row.ratio, rows[0].ratio)
  )
  return {
    source: reference.source,
    rows,
    uniformRatio: uniform && !pendingCache ? rows[0].ratio : undefined,
    pendingCache,
  }
}
