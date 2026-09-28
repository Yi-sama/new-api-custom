import { api } from '@/lib/api'

export type ChannelStatusCard = {
  channel_id: number
  channel_name: string
  channel_type: number
  status: 'normal' | 'degraded' | 'failed' | 'unknown'
  availability_24h: number
  latest_latency_ms: number
  average_latency_ms: number
  blocks: Array<{ started_at: number; status: string }>
  last_checked_at: number
  error?: string
}

export type ChannelStatusOverview = {
  enabled: boolean
  mode: 'user' | 'api' | 'user_then_api'
  checked_at: number
  groups: Array<{ group: string; models?: Array<{ model: string; cards?: ChannelStatusCard[] }> }>
}

export type ChannelStatusGroupOption = {
  group: string
  models: string[]
}

export async function getChannelStatusOverview() {
  const response = await api.get<{ success: boolean; data: ChannelStatusOverview }>('/api/channel-status/overview')
  return response.data
}

export async function getChannelStatusGroups() {
  const response = await api.get<{ success: boolean; data: { items: ChannelStatusGroupOption[] } }>('/api/channel-status/groups')
  return response.data
}
