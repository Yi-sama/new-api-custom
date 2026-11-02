import { api } from '@/lib/api'

export type DailyTokenPoolConfig = {
  enabled: boolean
  free_usage: boolean
  title: string
  daily_limit: number
  reset_timezone: string
  groups: string[]
  channel_ids: number[]
  public_visible: boolean
}

export type DailyTokenPoolStatus =
  | { enabled: false }
  | {
      enabled: true
      title: string
      daily_limit: number
      used_tokens: number
      reserved_tokens: number
      remaining_tokens: number
      reset_at: number
      reset_timezone: string
      exhausted: boolean
    }

export type PoolAdminData = {
  config: DailyTokenPoolConfig
  channels: { id: number; name: string }[]
}

type Response<T> = { success: boolean; data: T; message?: string }

function normalizeDailyTokenPoolConfig(
  config: DailyTokenPoolConfig
): DailyTokenPoolConfig {
  return { ...config, free_usage: config.free_usage ?? false }
}

export async function getDailyTokenPool(): Promise<DailyTokenPoolStatus> {
  const response = await api.get<Response<DailyTokenPoolStatus>>(
    '/api/daily-token-pool'
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to load shared token pool')
  }
  return response.data.data
}

export async function getDailyTokenPoolAdmin(): Promise<PoolAdminData> {
  const response = await api.get<Response<PoolAdminData>>(
    '/api/daily-token-pool/admin'
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to load shared token pool')
  }
  return {
    ...response.data.data,
    config: normalizeDailyTokenPoolConfig(response.data.data.config),
  }
}

export async function saveDailyTokenPool(
  config: DailyTokenPoolConfig
): Promise<DailyTokenPoolConfig> {
  const response = await api.put<Response<DailyTokenPoolConfig>>(
    '/api/daily-token-pool/admin',
    config
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to save shared token pool')
  }
  return normalizeDailyTokenPoolConfig(response.data.data)
}
