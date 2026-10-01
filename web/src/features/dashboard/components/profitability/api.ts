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

export type ProfitabilityRevenueBasis = 'calls' | 'cash'

export type ProfitabilityStatus =
  | 'empty_selection'
  | 'not_configured'
  | 'sync_error'
  | 'unmatched'
  | 'missing_purchase'
  | 'no_activity'
  | 'complete'

export interface ProfitabilityAvailableChannel {
  id: number
  name: string
  status: number
}

export interface ProfitabilityCallComparison {
  charge_usd: number
  sub_usage_usd: number | null
  effective_ratio: number | null
  status: string
}

export interface ProfitabilityGroupCallComparison extends ProfitabilityCallComparison {
  break_even_ratio: number | null
  break_even_status: string
}

export interface ProfitabilityPoolSummary {
  all_calls: ProfitabilityCallComparison
  customer_calls: ProfitabilityCallComparison
  admin_charge_usd: number
  matched_admin_sub_usage_usd: number
  outside_selected_sub_usage_usd: number
  profit_rmb: number | null
  profit_status: string
}

export interface ProfitabilityReport {
  pool?: ProfitabilityPoolSummary
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
  available_channels: ProfitabilityAvailableChannel[]
  scope: 'selected' | 'all'
  selected_channel_ids: number[]
  profit_status: ProfitabilityStatus
  matched_requests: number
  unmatched_requests: number
  unmatched_sub_usage_usd: number
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
  status: number
  profit_status: ProfitabilityStatus
  purchase_recorded: boolean
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
  profit_status?: ProfitabilityStatus
}

export interface ProfitabilityAccount {
  channel_id: number
  account_id: number
  sub_usage_usd: number
}

export interface ProfitabilityGroup {
  name: string
  ratio: number
  configured_ratio?: number | null
  customer_usd: number
  self_usd: number
  profitable: boolean
  profit_status:
    | ProfitabilityStatus
    | 'profit'
    | 'loss'
    | 'break_even'
    | 'unknown'
  sub_usage_usd?: number
  allocated_cost_rmb?: number
  customer_charge_rmb?: number
  estimated_profit_rmb?: number | null
  break_even_ratio?: number | null
  all_calls?: ProfitabilityGroupCallComparison
  customer_calls?: ProfitabilityGroupCallComparison
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

export async function getProfitabilitySummary(
  range: ProfitabilityTimeRange,
  revenueBasis: ProfitabilityRevenueBasis = 'calls',
  channelIds?: number[]
): Promise<ProfitabilityReport> {
  const normalizedChannelIds = channelIds
    ? [...new Set(channelIds)].sort((left, right) => left - right)
    : undefined
  const res = await api.get('/api/profitability/summary', {
    params: {
      start_timestamp: range.startTimestamp || undefined,
      end_timestamp: range.endTimestamp || undefined,
      revenue_basis: revenueBasis,
      channel_ids:
        revenueBasis === 'calls' && normalizedChannelIds !== undefined
          ? normalizedChannelIds.join(',')
          : undefined,
    },
  })
  if (!res.data?.success || !res.data?.data) {
    throw new Error(res.data?.message || 'Failed to load profitability report')
  }
  return res.data.data
}

export async function addProfitabilityEntry(
  request: ProfitabilityEntryRequest
): Promise<void> {
  await api.post('/api/profitability/entries', request)
}
