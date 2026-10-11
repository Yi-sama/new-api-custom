import { expect, test } from 'vitest'

import { defaultStatusImageConfig, statusImageSchema } from '../config'
import {
  channelReport,
  channelReportComparisons,
  channelReportPages,
  channelReportSections,
  filterSubReport,
  sampleSubReport,
} from '../data'

test('keeps missing samples distinct from measured zero-percent availability', () => {
  const data = channelReport({
    enabled: true,
    mode: 'user',
    checked_at: 1,
    groups: [
      {
        group: 'default',
        models: [
          { model: 'empty', cards: [] },
          {
            model: 'failed',
            cards: [
              {
                channel_id: 1,
                channel_name: 'private name',
                channel_type: 1,
                status: 'failed',
                availability_24h: 0,
                latest_latency_ms: 1234,
                average_latency_ms: 0,
                last_checked_at: 100,
                blocks: [],
                error: 'private error',
              },
            ],
          },
        ],
      },
    ],
  })
  expect(data.groups[0].items[0]).toMatchObject({
    availability: null,
    latency: null,
    status: 'unknown',
  })
  expect(data.groups[0].items[1]).toMatchObject({
    availability: 0,
    latency: 1234,
    status: 'failed',
  })
  expect(JSON.stringify(data)).not.toContain('private')
})

test('bounds report text and layout while accepting an empty group filter', () => {
  expect(statusImageSchema.safeParse(defaultStatusImageConfig).success).toBe(
    true
  )
  expect(
    statusImageSchema.safeParse({
      ...defaultStatusImageConfig,
      site_name: 'x'.repeat(61),
    }).success
  ).toBe(false)
  expect(
    statusImageSchema.safeParse({
      ...defaultStatusImageConfig,
      channel_columns: 3,
    }).success
  ).toBe(false)
})

test('blank Sub filtering retains all groups and an exact filter removes unrelated accounts', () => {
  const report = {
    status: 'connected',
    checked_at: 1,
    groups: [
      { name: 'A', ready: 1, total: 1, cooling: 0, error: 0 },
      { name: 'B', ready: 0, total: 1, cooling: 1, error: 0 },
    ],
    accounts: [
      { id: 1, name: 'OpenAI #1', groups: ['A'], status: 'normal' },
      { id: 2, name: 'OpenAI #2', groups: ['B'], status: 'cooling' },
    ],
  }
  expect(filterSubReport(report, '').groups).toHaveLength(2)
  expect(filterSubReport(report, ' B ').accounts).toEqual([report.accounts[1]])
  expect(filterSubReport(report, 'missing').accounts).toEqual([])
})

test('Sub sample group counts agree with the actual fixture accounts', () => {
  for (const group of sampleSubReport.groups) {
    const accounts = sampleSubReport.accounts.filter((account) =>
      account.groups.includes(group.name)
    )
    expect(group.total).toBe(accounts.length)
    expect(group.ready).toBe(
      accounts.filter((account) => account.status === 'normal').length
    )
    expect(group.cooling).toBe(
      accounts.filter((account) => account.status === 'cooling').length
    )
    expect(group.error).toBe(
      accounts.filter((account) => account.status === 'failed').length
    )
  }
})

test('channel pagination retains every item beyond sixty and filters before paginating', () => {
  const item = {
    name: 'model',
    status: 'unknown',
    availability: null,
    latency: null,
    checkedAt: 0,
    history: [],
  }
  const report = {
    enabled: true,
    groups: [
      { name: 'A', items: Array.from({ length: 65 }, () => ({ ...item })) },
    ],
  }
  const pages = channelReportPages(report, '')
  expect(pages).toHaveLength(4)
  expect(
    pages.flatMap((page) => page.groups.flatMap((group) => group.items))
  ).toHaveLength(65)
  expect(channelReportPages(report, 'missing')[0].groups).toEqual([])
})

test('mixed-provider groups appear under each provider without losing unknown models', () => {
  const item = {
    status: 'unknown',
    availability: null,
    latency: null,
    checkedAt: 0,
    history: [],
  }
  const sections = channelReportSections(
    {
      enabled: true,
      groups: [
        {
          name: 'Shared pool',
          items: [
            { ...item, name: 'gpt-5' },
            { ...item, name: 'claude-sonnet' },
            { ...item, name: 'custom-model' },
            { ...item, name: 'codex-auto-review' },
          ],
        },
        { name: 'Empty pool', items: [] },
      ],
    },
    ''
  )
  expect(sections.map((section) => section.provider)).toEqual([
    'OpenAI',
    'Anthropic',
    'Other',
  ])
  expect(sections[0].groups[0].items.map((model) => model.name)).toEqual([
    'gpt-5',
    'codex-auto-review',
  ])
  expect(sections[1].groups[0].name).toBe('Shared pool')
  expect(sections[2].groups.map((group) => group.name)).toEqual([
    'Shared pool',
    'Empty pool',
  ])
})

test('pagination moves a complete short group to the next page instead of splitting it', () => {
  const item = {
    name: 'gpt-5',
    status: 'unknown',
    availability: null,
    latency: null,
    checkedAt: 0,
    history: [],
  }
  const pages = channelReportPages(
    {
      enabled: true,
      groups: [
        { name: 'First pool', items: Array(15).fill(item) },
        { name: 'Second pool', items: Array(10).fill(item) },
      ],
    },
    ''
  )
  expect(pages.map((page) => page.groups.map((group) => group.name))).toEqual([
    ['First pool'],
    ['Second pool'],
  ])
  expect(pages[1].groups[0].items).toHaveLength(10)
})

test('model comparisons retain distinct upstream samples inside their original group', () => {
  const item = {
    name: 'gpt-5',
    status: 'normal',
    availability: 100,
    latency: 1200,
    checkedAt: 100,
    history: [],
  }
  const models = channelReportComparisons({
    provider: 'OpenAI',
    groups: [
      {
        name: 'A',
        items: [
          item,
          { ...item, status: 'failed', availability: 0 },
          { ...item, name: 'gpt-mini' },
        ],
      },
      { name: 'B', items: [{ ...item, availability: 98.3 }] },
    ],
  })
  expect(models.map((model) => model.name)).toEqual(['gpt-5', 'gpt-mini'])
  expect(models[0].groups.map((group) => group.name)).toEqual(['A', 'B'])
  expect(
    models[0].groups[0].items.map((sample) => sample.availability)
  ).toEqual([100, 0])
  expect(models[0].groups[1].items[0].availability).toBe(98.3)
})

test('pagination moves a complete model comparison instead of splitting its groups at a page boundary', () => {
  const item = {
    name: 'gpt-5',
    status: 'unknown',
    availability: null,
    latency: null,
    checkedAt: 0,
    history: [],
  }
  const pages = channelReportPages(
    {
      enabled: true,
      groups: [
        {
          name: 'Previous',
          items: Array(17).fill({ ...item, name: 'gpt-previous' }),
        },
        { name: 'A', items: [item] },
        { name: 'B', items: [item] },
      ],
    },
    ''
  )
  expect(pages).toHaveLength(2)
  expect(pages[1].groups.map((group) => group.name)).toEqual(['A', 'B'])
})

test('exact model selections filter group-model pairs before pagination without dropping repeated samples', () => {
  const item = {
    name: 'gpt-5',
    status: 'unknown',
    availability: null,
    latency: null,
    checkedAt: 0,
    history: [],
  }
  const report = {
    enabled: true,
    groups: [
      {
        name: 'A',
        items: [...Array(20).fill(item), { ...item, name: 'claude-sonnet' }],
      },
      { name: 'B', items: [item, { ...item, name: 'claude-sonnet' }] },
      { name: 'Empty', items: [] },
    ],
  }
  const selection = [
    { group: 'A', model: 'gpt-5' },
    { group: 'B', model: 'claude-sonnet' },
    { group: 'Missing', model: 'gpt-5' },
  ]
  const pages = channelReportPages(report, '', selection)
  expect(pages).toHaveLength(2)
  expect(
    pages.flatMap((page) => page.groups).map((group) => group.name)
  ).toEqual(['A', 'A', 'B'])
  expect(
    pages
      .flatMap((page) => page.groups.flatMap((group) => group.items))
      .map((item) => item.name)
  ).toEqual([...Array(20).fill('gpt-5'), 'claude-sonnet'])
  expect(
    channelReportSections(report, 'B', selection)[0].groups[0].items.map(
      (item) => item.name
    )
  ).toEqual(['claude-sonnet'])
  expect(channelReportPages(report, '', [])[0].groups).toEqual([])
  expect(
    channelReportPages(report, '', [{ group: 'Empty', model: '-' }])[0].groups
  ).toEqual([])
  expect(
    channelReportPages(report, '', [{ group: 'A', model: 'missing' }])[0].groups
  ).toEqual([])
  expect(channelReportSections(report, '', null)).toEqual(
    channelReportSections(report, '')
  )
})
