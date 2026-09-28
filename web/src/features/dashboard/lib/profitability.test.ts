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
*/
import { describe, expect, test } from 'vitest'

import { calculateProfitability, type ProfitabilityInputs } from './profitability'

const inputs: ProfitabilityInputs = {
  activeUsers: 100,
  usagePerUser: 1000,
  usageUnit: 'credits',
  pricePerUser: 29,
  capacityPerAccount: 30_000,
  usageCostPerUnit: 0.01,
  accountMonthlyCost: 99,
  fixedMonthlyCost: 500,
}

describe('profitability calculator', () => {
  test('converts users and usage into account count and monthly profit', () => {
    expect(calculateProfitability(inputs)).toMatchObject({
      totalUsage: 100_000,
      requiredAccounts: 4,
      revenue: 2_900,
      variableUsageCost: 1_000,
      accountCost: 396,
      totalCost: 1_896,
      profit: 1_004,
    })
  })

  test('finds the first profitable user count across account capacity bands', () => {
    const result = calculateProfitability({
      ...inputs,
      activeUsers: 1,
      pricePerUser: 20,
      usagePerUser: 1,
      capacityPerAccount: 10,
      usageCostPerUnit: 1,
      accountMonthlyCost: 50,
      fixedMonthlyCost: 0,
    })

    expect(result.contributionPerUser).toBe(19)
    expect(result.breakEvenUsers).toBe(3)
    expect(result.breakEvenAccounts).toBe(1)
  })

  test('reports no break-even point when variable usage cost exceeds price', () => {
    const result = calculateProfitability({
      ...inputs,
      pricePerUser: 5,
      usagePerUser: 1000,
      usageCostPerUnit: 0.01,
    })

    expect(result.profit).toBeLessThan(0)
    expect(result.breakEvenUsers).toBeNull()
    expect(result.breakEvenAccounts).toBeNull()
  })

  test('normalizes invalid capacity and blank unit without throwing', () => {
    const result = calculateProfitability({
      ...inputs,
      usageUnit: ' ',
      capacityPerAccount: 0,
    })

    expect(result.requiredAccounts).toBe(100_000)
  })
})
