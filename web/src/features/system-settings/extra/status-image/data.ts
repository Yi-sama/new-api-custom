import type { ChannelStatusOverview } from '@/features/channel-status/api'

export type ReportItem = {
  name: string
  status: string
  availability: number | null
  latency: number | null
  checkedAt: number
  history: string[]
}
export type ReportGroup = { name: string; items: ReportItem[] }
export type ChannelReport = { enabled: boolean; groups: ReportGroup[] }

export function channelReport(overview: ChannelStatusOverview): ChannelReport {
  return {
    enabled: overview.enabled,
    groups: overview.groups.map((group) => ({
      name: group.group,
      items: (group.models ?? []).flatMap<ReportItem>((model) => {
        if (!model.cards?.length) {
          return [
            {
              name: model.model,
              status: 'unknown',
              availability: null,
              latency: null,
              checkedAt: 0,
              history: [],
            },
          ]
        }
        return model.cards.map((card) => ({
          name: model.model,
          status: card.status,
          availability: card.last_checked_at > 0 ? card.availability_24h : null,
          latency:
            card.last_checked_at > 0 && card.latest_latency_ms > 0
              ? card.latest_latency_ms
              : null,
          checkedAt: card.last_checked_at,
          history: card.blocks.map((block) => block.status),
        }))
      }),
    })),
  }
}

// Deliberately fixed fixtures. Every export using these carries a sample watermark.
export const sampleChannelReport: ChannelReport = {
  enabled: true,
  groups: [
    {
      name: 'Claude',
      items: [
        {
          name: 'claude-sonnet',
          status: 'normal',
          availability: 99.8,
          latency: 1420,
          checkedAt: 1789968000,
          history: [
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
          ],
        },
        {
          name: 'claude-opus',
          status: 'degraded',
          availability: 68.5,
          latency: 3860,
          checkedAt: 1789968000,
          history: [
            'normal',
            'normal',
            'failed',
            'normal',
            'normal',
            'unknown',
            'normal',
            'degraded',
            'normal',
            'normal',
            'normal',
            'degraded',
          ],
        },
      ],
    },
    {
      name: 'OpenAI',
      items: [
        {
          name: 'gpt-5',
          status: 'normal',
          availability: 100,
          latency: 980,
          checkedAt: 1789968000,
          history: [
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
            'normal',
          ],
        },
        {
          name: 'gpt-5-mini',
          status: 'unknown',
          availability: null,
          latency: null,
          checkedAt: 0,
          history: [],
        },
      ],
    },
  ],
}

export const sampleSubGroups = [
  { name: 'Claude', ready: 6, total: 8, cooling: 1, error: 1 },
  { name: 'Gemini', ready: 4, total: 4, cooling: 0, error: 0 },
]

export const sampleSubAccounts = [
  { name: 'Claude 01', group: 'Claude', status: 'normal' },
  { name: 'Claude 07', group: 'Claude', status: 'cooling' },
  { name: 'Claude 08', group: 'Claude', status: 'failed' },
  { name: 'Gemini 01', group: 'Gemini', status: 'normal' },
]
