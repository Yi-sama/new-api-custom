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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import { parseQuotaFromDollars } from '@/lib/format'

import {
  formValuesToPlanPayload,
  getPlanFormSchema,
  PLAN_FORM_DEFAULTS,
  planToFormValues,
} from '../plan-form'

const t = ((key: string) => key) as TFunction

describe('subscription plan form', () => {
  test('defaults to full quota allocation', () => {
    expect(PLAN_FORM_DEFAULTS).toMatchObject({
      quota_grant_mode: 'full',
      daily_amount: 0,
      quota_expiry_policy: 'expire',
    })
  })

  test('requires positive lifetime and daily amounts for daily accrual', () => {
    const result = getPlanFormSchema(t).safeParse({
      ...PLAN_FORM_DEFAULTS,
      title: 'Daily',
      quota_grant_mode: 'daily',
      total_amount: 0,
      daily_amount: 0,
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map((issue) => issue.path[0])).toEqual(
        expect.arrayContaining(['total_amount', 'daily_amount'])
      )
    }
  })

  test('round trips daily fields and forces reset off in the payload', () => {
    const values = planToFormValues({
      id: 3,
      title: 'Daily',
      price_amount: 5,
      currency: 'USD',
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'daily',
      quota_grant_mode: 'daily',
      daily_amount: parseQuotaFromDollars(2),
      quota_expiry_policy: 'retain',
      enabled: true,
      sort_order: 0,
      allow_balance_pay: true,
      allow_wallet_overflow: true,
      max_purchase_per_user: 0,
      total_amount: parseQuotaFromDollars(20),
    })
    const payload = formValuesToPlanPayload({
      ...values,
      quota_reset_period: 'monthly',
    })

    expect(values).toMatchObject({
      quota_grant_mode: 'daily',
      daily_amount: 2,
      quota_expiry_policy: 'retain',
      total_amount: 20,
    })
    expect(payload.plan).toMatchObject({
      quota_grant_mode: 'daily',
      daily_amount: parseQuotaFromDollars(2),
      quota_expiry_policy: 'retain',
      quota_reset_period: 'never',
      quota_reset_custom_seconds: 0,
      total_amount: parseQuotaFromDollars(20),
    })
  })
})
