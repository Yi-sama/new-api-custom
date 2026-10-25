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
import { describe, expect, test } from 'vitest'

import { getSubscriptionQuotaState } from '../subscription-quota'

describe('subscription quota state', () => {
  test('uses granted amount as the daily unlocked balance', () => {
    expect(
      getSubscriptionQuotaState(
        {
          amount_total: 100,
          amount_used: 7,
          amount_granted: 40,
          quota_grant_mode: 'daily',
          quota_expiry_policy: 'expire',
          end_time: 200,
          status: 'active',
        },
        100
      )
    ).toMatchObject({
      earnedAmount: 40,
      availableAmount: 33,
      usagePercent: 18,
      isActive: true,
    })
  })

  test('uses total entitlement for legacy daily snapshots without grant data', () => {
    expect(
      getSubscriptionQuotaState(
        {
          amount_total: 100,
          amount_used: 7,
          quota_grant_mode: 'daily',
          quota_expiry_policy: 'expire',
          end_time: 200,
          status: 'active',
        },
        100
      )
    ).toMatchObject({ earnedAmount: 100, availableAmount: 93 })
  })

  test('keeps only expired daily retained balances spendable at the expiry boundary', () => {
    const retained = getSubscriptionQuotaState(
      {
        amount_total: 100,
        amount_used: 25,
        amount_granted: 50,
        quota_grant_mode: 'daily',
        quota_expiry_policy: 'retain',
        end_time: 100,
        status: 'active',
      },
      100
    )
    const fullQuota = getSubscriptionQuotaState(
      {
        amount_total: 100,
        amount_used: 25,
        quota_grant_mode: 'full',
        quota_expiry_policy: 'retain',
        end_time: 100,
        status: 'active',
      },
      100
    )

    expect(retained).toMatchObject({
      isExpired: true,
      hasRetainedBalance: true,
    })
    expect(fullQuota.hasRetainedBalance).toBe(false)
  })
})
