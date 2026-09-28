import { api } from '@/lib/api'

export interface ProfitabilityRevenueItem {
  id: number
  source: string
  source_id: string
  amount_usd: number
  user_id: number
  username: string
  note: string
  occurred_at: number
}

export interface ProfitabilityEntry {
  id: number
  entry_type: string
  amount_usd: number
  amount_rmb: number
  account_count: number
  quota: number
  user_id: number
  username: string
  source: string
  source_id: string
  note: string
  occurred_at: number
}

export interface ProfitabilityUsageUser {
  user_id: number
  username: string
  quota: number
  amount_usd: number
  request_count: number
  token_count: number
}

export interface ProfitabilityReport {
  revenue_usd: number
  topup_revenue_usd: number
  manual_income_usd: number
  admin_adjustment_usd: number
  account_cost_rmb: number
  sub_usage_usd: number
  cost_rate_rmb_per_usd: number
  account_count: number
  account_purchase_count: number
  sub_usage_status: string
  sub_usage_error?: string
  channels: ProfitabilityChannel[]
  models: ProfitabilityModel[]
  accounts: ProfitabilityAccount[]
  unmatched_sub_requests: number
  cash_received_rmb: number
  cash_recognized_rmb: number
  cash_profit_rmb: number | null
  call_charge_rmb: number
  estimated_profit_rmb: number | null
  groups: ProfitabilityGroup[]
  usage: {
    quota: number
    amount_usd: number
    customer_usd: number
    self_usd: number
    request_count: number
    token_count: number
    users: ProfitabilityUsageUser[]
    groups: ProfitabilityGroup[]
  }
}

export interface ProfitabilityChannel {
  id: number
  name: string
  purchase_rmb: number
  sub_usage_usd: number
  customer_charge_rmb: number
  estimated_profit_rmb: number | null
  matched_requests: number
  unmatched_requests: number
}

export interface ProfitabilityModel {
  channel_id: number
  name: string
  sub_usage_usd: number
  customer_charge_rmb: number
  allocated_cost_rmb: number
  estimated_profit_rmb: number | null
}

export interface ProfitabilityAccount {
  channel_id: number
  account_id: number
  sub_usage_usd: number
}

export interface ProfitabilityGroup {
  name: string
  ratio: number
  customer_usd: number
  self_usd: number
  profitable: boolean
  profit_status: string
  sub_usage_usd?: number
  allocated_cost_rmb?: number
  customer_charge_rmb?: number
  estimated_profit_rmb?: number | null
  break_even_ratio?: number | null
}

export interface ProfitabilityTimeRange {
  startTimestamp: number
  endTimestamp: number
}

export interface ProfitabilityEntryRequest {
  entry_type: 'account_purchase' | 'manual_income'
  amount_usd: number
  amount_rmb?: number
  account_count?: number
  channel_id?: number
  note?: string
}

const EMPTY_REPORT: ProfitabilityReport = {
  revenue_usd: 0,
  topup_revenue_usd: 0,
  manual_income_usd: 0,
  admin_adjustment_usd: 0,
  account_cost_rmb: 0,
  sub_usage_usd: 0,
  cost_rate_rmb_per_usd: 0,
  account_count: 0,
  account_purchase_count: 0,
  sub_usage_status: 'not_configured',
  channels: [],
  models: [],
  accounts: [],
  unmatched_sub_requests: 0,
  cash_received_rmb: 0,
  cash_recognized_rmb: 0,
  cash_profit_rmb: null,
  call_charge_rmb: 0,
  estimated_profit_rmb: null,
  groups: [],
  usage: {
    quota: 0,
    amount_usd: 0,
    customer_usd: 0,
    self_usd: 0,
    request_count: 0,
    token_count: 0,
    users: [],
    groups: [],
  },
}

export async function getProfitabilitySummary(
  range: ProfitabilityTimeRange
): Promise<ProfitabilityReport> {
  const res = await api.get('/api/profitability/summary', {
    params: {
      start_timestamp: range.startTimestamp || undefined,
      end_timestamp: range.endTimestamp || undefined,
    },
  })
  return res.data?.data ?? EMPTY_REPORT
}

export async function addProfitabilityEntry(
  request: ProfitabilityEntryRequest
): Promise<void> {
  await api.post('/api/profitability/entries', request)
}
