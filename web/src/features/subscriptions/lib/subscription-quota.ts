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
import type { UserSubscription } from '../types'

export function getSubscriptionQuotaState(
  subscription: Pick<
    UserSubscription,
    | 'amount_total'
    | 'amount_used'
    | 'amount_granted'
    | 'quota_grant_mode'
    | 'quota_expiry_policy'
    | 'end_time'
    | 'status'
  >,
  now = Date.now() / 1000
) {
  const totalAmount = Math.max(0, Number(subscription.amount_total || 0))
  const usedAmount = Math.max(0, Number(subscription.amount_used || 0))
  const isDaily = subscription.quota_grant_mode === 'daily'
  const earnedAmount = totalAmount
    ? Math.min(
        totalAmount,
        Math.max(
          0,
          Number(
            isDaily ? (subscription.amount_granted ?? totalAmount) : totalAmount
          )
        )
      )
    : 0
  const availableAmount = totalAmount
    ? Math.max(0, earnedAmount - usedAmount)
    : 0
  const isExpired =
    Number(subscription.end_time || 0) > 0 && subscription.end_time <= now
  const isActive = subscription.status === 'active' && !isExpired
  const hasRetainedBalance =
    isDaily &&
    subscription.quota_expiry_policy === 'retain' &&
    isExpired &&
    (subscription.status === 'active' || subscription.status === 'expired') &&
    availableAmount > 0

  return {
    totalAmount,
    usedAmount,
    earnedAmount,
    availableAmount,
    isDaily,
    isExpired,
    isActive,
    hasRetainedBalance,
    usagePercent:
      earnedAmount > 0 ? Math.round((usedAmount / earnedAmount) * 100) : 0,
  }
}
