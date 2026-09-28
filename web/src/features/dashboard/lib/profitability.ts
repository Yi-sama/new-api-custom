/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

export interface ProfitabilityInputs {
  activeUsers: number
  usagePerUser: number
  usageUnit: string
  pricePerUser: number
  capacityPerAccount: number
  usageCostPerUnit: number
  accountMonthlyCost: number
  fixedMonthlyCost: number
}

export interface ProfitabilityResult {
  totalUsage: number
  requiredAccounts: number
  revenue: number
  variableUsageCost: number
  accountCost: number
  totalCost: number
  profit: number
  profitMargin: number | null
  contributionPerUser: number
  breakEvenUsers: number | null
  breakEvenAccounts: number | null
}

function nonNegative(value: number, fallback = 0): number {
  return Number.isFinite(value) ? Math.max(fallback, value) : fallback
}

function findBreakEvenUsers(
  inputs: ProfitabilityInputs,
  contributionPerUser: number
): number | null {
  if (inputs.fixedMonthlyCost === 0 && inputs.accountMonthlyCost === 0) {
    return contributionPerUser >= 0 ? 0 : null
  }
  if (contributionPerUser <= 0) return null

  if (inputs.usagePerUser === 0) {
    return Math.ceil(inputs.fixedMonthlyCost / contributionPerUser)
  }

  // Check each account-capacity band because opening another account adds a
  // fixed cost and can temporarily lower the margin.
  for (let accountCount = 1; accountCount <= 100_000; accountCount += 1) {
    const lowerUserBound =
      Math.floor(
        ((accountCount - 1) * inputs.capacityPerAccount) /
          inputs.usagePerUser
      ) + 1
    const upperUserBound = Math.floor(
      (accountCount * inputs.capacityPerAccount) / inputs.usagePerUser
    )
    if (upperUserBound < lowerUserBound) continue

    const candidate = Math.max(
      lowerUserBound,
      Math.ceil(
        (inputs.fixedMonthlyCost + accountCount * inputs.accountMonthlyCost) /
          contributionPerUser
      )
    )
    if (candidate <= upperUserBound) return candidate
  }

  return null
}

export function calculateProfitability(
  rawInputs: ProfitabilityInputs
): ProfitabilityResult {
  const inputs: ProfitabilityInputs = {
    ...rawInputs,
    activeUsers: nonNegative(rawInputs.activeUsers),
    usagePerUser: nonNegative(rawInputs.usagePerUser),
    usageUnit: rawInputs.usageUnit.trim() || 'units',
    pricePerUser: nonNegative(rawInputs.pricePerUser),
    capacityPerAccount: Math.max(
      1,
      nonNegative(rawInputs.capacityPerAccount, 1)
    ),
    usageCostPerUnit: nonNegative(rawInputs.usageCostPerUnit),
    accountMonthlyCost: nonNegative(rawInputs.accountMonthlyCost),
    fixedMonthlyCost: nonNegative(rawInputs.fixedMonthlyCost),
  }

  const totalUsage = inputs.activeUsers * inputs.usagePerUser
  const requiredAccounts =
    totalUsage > 0 ? Math.ceil(totalUsage / inputs.capacityPerAccount) : 0
  const revenue = inputs.activeUsers * inputs.pricePerUser
  const variableUsageCost = totalUsage * inputs.usageCostPerUnit
  const accountCost = requiredAccounts * inputs.accountMonthlyCost
  const totalCost = variableUsageCost + accountCost + inputs.fixedMonthlyCost
  const profit = revenue - totalCost
  const contributionPerUser =
    inputs.pricePerUser -
    inputs.usagePerUser * inputs.usageCostPerUnit
  const breakEvenUsers = findBreakEvenUsers(inputs, contributionPerUser)
  let breakEvenAccounts: number | null
  if (breakEvenUsers === null) {
    breakEvenAccounts = null
  } else if (breakEvenUsers > 0) {
    breakEvenAccounts = Math.ceil(
      (breakEvenUsers * inputs.usagePerUser) / inputs.capacityPerAccount
    )
  } else {
    breakEvenAccounts = 0
  }

  return {
    totalUsage,
    requiredAccounts,
    revenue,
    variableUsageCost,
    accountCost,
    totalCost,
    profit,
    profitMargin: revenue > 0 ? profit / revenue : null,
    contributionPerUser,
    breakEvenUsers,
    breakEvenAccounts,
  }
}
