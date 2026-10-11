import type { ChannelStatusOverview } from '@/features/channel-status/api'
import { api } from '@/lib/api'

import type { ReportKind, StatusImageConfig } from './config'
import type { SubReport } from './data'

export async function getStatusImageOverview(kind: ReportKind) {
  const response = await api.get<{
    success: boolean
    data: {
      config: StatusImageConfig
      channel?: ChannelStatusOverview
      sub?: SubReport
    }
  }>('/api/status-images/overview', { params: { kind } })
  return response.data
}
