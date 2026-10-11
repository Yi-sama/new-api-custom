import type { ChannelStatusOverview } from '@/features/channel-status/api'

import type { ChannelModelSelection } from './config'

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
export type ChannelReportSection = { provider: string; groups: ReportGroup[] }
export type ReportModel = { name: string; groups: ReportGroup[] }
export type SubReport = {
  status: string
  error?: string
  checked_at: number
  groups: Array<{
    name: string
    ready: number
    total: number
    cooling: number
    error: number
  }>
  accounts: Array<{
    id: number
    name: string
    groups: string[]
    status: string
  }>
}

export function filterSubReport(report: SubReport, filter: string): SubReport {
  const names = new Set(
    filter
      .split(/[,\uFF0C\n]/)
      .map((name) => name.trim())
      .filter(Boolean)
  )
  const groups = report.groups.filter(
    (group) => names.size === 0 || names.has(group.name)
  )
  const visible = new Set(groups.map((group) => group.name))
  return {
    ...report,
    groups,
    accounts: report.accounts.filter((account) =>
      account.groups.some((group) => visible.has(group))
    ),
  }
}

const providerModels = [
  ['Anthropic', /claude|anthropic/i],
  ['Google', /gemini|gemma/i],
  ['DeepSeek', /deepseek/i],
  ['Qwen', /qwen|qwq/i],
  ['xAI', /grok|xai/i],
  ['OpenAI', /\b(gpt|codex|chatgpt|dall-e|text-embedding|o1|o3|o4)\b/i],
] as const

export function channelReportSections(
  report: ChannelReport,
  filter: string,
  models?: ChannelModelSelection | null
): ChannelReportSection[] {
  const names = new Set(
    filter
      .split(/[,\uFF0C\n]/)
      .map((name) => name.trim())
      .filter(Boolean)
  )
  const selected =
    models == null
      ? null
      : new Set(models.map((item) => JSON.stringify([item.group, item.model])))
  const sections = new Map<string, ChannelReportSection>()
  for (const group of report.groups) {
    if (names.size && !names.has(group.name)) continue
    const items =
      group.items.length || selected !== null
        ? group.items
        : [
            {
              name: '-',
              status: 'unknown',
              availability: null,
              latency: null,
              checkedAt: 0,
              history: [],
            },
          ]
    for (const item of items) {
      if (selected && !selected.has(JSON.stringify([group.name, item.name]))) {
        continue
      }
      const provider =
        providerModels.find(([, pattern]) => pattern.test(item.name))?.[0] ??
        'Other'
      let section = sections.get(provider)
      if (!section) {
        section = { provider, groups: [] }
        sections.set(provider, section)
      }
      let target = section.groups.find((entry) => entry.name === group.name)
      if (!target) {
        target = { name: group.name, items: [] }
        section.groups.push(target)
      }
      target.items.push(item)
    }
  }
  const order = [
    'OpenAI',
    'Anthropic',
    'Google',
    'DeepSeek',
    'Qwen',
    'xAI',
    'Other',
  ]
  return [...sections.values()].sort(
    (left, right) =>
      order.indexOf(left.provider) - order.indexOf(right.provider)
  )
}

export function channelReportPages(
  report: ChannelReport,
  filter: string,
  models?: ChannelModelSelection | null
): ChannelReport[] {
  const pages: ChannelReport[] = []
  let page: ChannelReport = { enabled: report.enabled, groups: [] }
  let count = 0
  const pageSize = 18
  for (const section of channelReportSections(report, filter, models)) {
    for (const model of channelReportComparisons(section)) {
      const modelCount = model.groups.reduce(
        (sum, group) => sum + group.items.length,
        0
      )
      // Keep a model's comparison together when it fits on one page.
      if (count > 0 && count + Math.min(modelCount, pageSize) > pageSize) {
        pages.push(page)
        page = { enabled: report.enabled, groups: [] }
        count = 0
      }
      for (const group of model.groups) {
        if (
          count > 0 &&
          count + Math.min(group.items.length, pageSize) > pageSize
        ) {
          pages.push(page)
          page = { enabled: report.enabled, groups: [] }
          count = 0
        }
        for (let offset = 0; offset < group.items.length; offset += pageSize) {
          if (count === pageSize) {
            pages.push(page)
            page = { enabled: report.enabled, groups: [] }
            count = 0
          }
          const items = group.items.slice(offset, offset + pageSize)
          page.groups.push({ name: group.name, items })
          count += items.length
        }
      }
    }
  }
  pages.push(page)
  return pages
}

export function channelReportComparisons(
  section: ChannelReportSection
): ReportModel[] {
  const models = new Map<string, ReportModel>()
  for (const group of section.groups) {
    for (const item of group.items) {
      let model = models.get(item.name)
      if (!model) {
        model = { name: item.name, groups: [] }
        models.set(item.name, model)
      }
      let target = model.groups.find((entry) => entry.name === group.name)
      if (!target) {
        target = { name: group.name, items: [] }
        model.groups.push(target)
      }
      target.items.push(item)
    }
  }
  return [...models.values()]
}

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
  { name: 'Claude', ready: 1, total: 3, cooling: 1, error: 1 },
  { name: 'Gemini', ready: 1, total: 1, cooling: 0, error: 0 },
]

export const sampleSubAccounts = [
  { name: 'Claude 01', group: 'Claude', status: 'normal' },
  { name: 'Claude 07', group: 'Claude', status: 'cooling' },
  { name: 'Claude 08', group: 'Claude', status: 'failed' },
  { name: 'Gemini 01', group: 'Gemini', status: 'normal' },
]

export const sampleSubReport: SubReport = {
  status: 'connected',
  checked_at: 1789968000,
  groups: sampleSubGroups,
  accounts: sampleSubAccounts.map((account, index) => ({
    id: index + 1,
    name: account.name,
    groups: [account.group],
    status: account.status,
  })),
}
