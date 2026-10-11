import i18next from 'i18next'
import { expect, test } from 'vitest'

import { defaultStatusImageConfig, type StatusImageConfig } from '../config'
import type { ChannelReport, SubReport } from '../data'
import { renderStatusImage } from '../render'

function draw(
  report: ChannelReport,
  config: Partial<StatusImageConfig> = {},
  subReport?: SubReport
) {
  let labels: Array<{
    value: string
    x: number
    y: number
    width: number
    size: number
    font: string
  }> = []
  let frames: Array<{ x: number; y: number; width: number; height: number }> =
    []
  let fills: Array<{ color: string; width: number; height: number }> = []
  const context = {
    font: '',
    fillStyle: '',
    measureText: (value: string) => ({ width: value.length * 10 }),
    fillText: (value: string, x: number, y: number) =>
      labels.push({
        value,
        x,
        y,
        width: value.length * 10,
        size: Number(context.font.match(/([\d.]+)px/)?.[1]),
        font: context.font,
      }),
    strokeRect: (x: number, y: number, width: number, height: number) =>
      frames.push({ x, y, width, height }),
    roundRect: (x: number, y: number, width: number, height: number) =>
      frames.push({ x, y, width, height }),
    drawImage() {},
    fillRect(_x: number, _y: number, width: number, height: number) {
      fills.push({ color: context.fillStyle, width, height })
    },
    beginPath() {},
    arc() {},
    fill() {
      fills.push({ color: context.fillStyle, width: 0, height: 0 })
    },
    stroke() {},
    save() {},
    restore() {},
  }
  let height = 1
  const canvas = {
    width: 0,
    get height() {
      return height
    },
    set height(value: number) {
      height = value
      labels = []
      frames = []
      fills = []
    },
    getContext: () => context,
  }
  renderStatusImage(canvas as unknown as HTMLCanvasElement, {
    config: { ...defaultStatusImageConfig, ...config },
    kind: subReport ? 'sub' : 'channel',
    sample: false,
    report,
    subReport,
    theme: {
      getPropertyValue: (name: string) => {
        if (name === '--font-sans') return 'Arial, sans-serif'
        if (name === '--font-serif') return 'Georgia, serif'
        return '#222222'
      },
    } as unknown as CSSStyleDeclaration,
    logo: {} as HTMLImageElement,
    t: i18next.t,
  })
  return {
    labels,
    frames: frames.filter((frame) => frame.height > 60),
    fills,
    height,
    width: canvas.width,
  }
}

const item = {
  name: 'gpt-5',
  status: 'normal',
  availability: 100,
  latency: 1200,
  checkedAt: 100,
  history: ['normal', 'failed', 'unknown'],
}

test('channel export defaults to a black canvas even when the surrounding theme is light', () => {
  const output = draw({
    enabled: true,
    groups: [{ name: 'Pool', items: [item] }],
  })
  expect(output.fills[0]).toEqual({
    color: '#000000',
    width: 1200,
    height: output.height,
  })
  expect(output.labels.find((label) => label.value === 'Pool')?.font).toContain(
    '600 '
  )
})

test('both reports use A sans headings, cool neutral surfaces and fixed status colors', () => {
  for (const output of [
    draw({ enabled: true, groups: [{ name: 'Pool', items: [item] }] }),
    draw({ enabled: true, groups: [] }, {}, sub),
  ]) {
    expect(output.labels.every((label) => label.font.includes('Arial'))).toBe(
      true
    )
    expect(output.labels.some((label) => label.font.includes('Georgia'))).toBe(
      false
    )
    expect(output.fills.some((fill) => fill.color === '#121516')).toBe(true)
    expect(output.fills.some((fill) => fill.color === '#78c9a5')).toBe(true)
  }
})

test('compares the same model across group frames below unframed model and provider titles', () => {
  const output = draw({
    enabled: true,
    groups: [
      { name: 'Plus pool', items: [item] },
      {
        name: 'Stable pool',
        items: [{ ...item, availability: null, checkedAt: 0 }],
      },
    ],
  })
  expect(output.frames).toHaveLength(2)
  expect(output.frames[0].y).toBe(output.frames[1].y)
  const model = output.labels.find((label) => label.value === 'gpt-5')
  expect(model?.y).toBeLessThan(output.frames[0].y)
  const provider = output.labels.find((label) => label.value === 'OpenAI')
  expect(provider).toBeDefined()
  expect(provider?.y).toBeLessThan(output.frames[0].y)
  expect(
    output.labels.filter((label) => label.value === 'Plus pool')
  ).toHaveLength(1)
})

test('measured success rate is the large primary metric and missing samples never display a fabricated percentage', () => {
  const output = draw({
    enabled: true,
    groups: [
      { name: 'Measured', items: [{ ...item, availability: 98.3 }] },
      {
        name: 'Unsampled',
        items: [
          {
            ...item,
            status: 'unknown',
            availability: null,
            latency: null,
            checkedAt: 0,
          },
        ],
      },
    ],
  })
  expect(
    output.labels.find((label) => label.value === '98.3%')?.size
  ).toBeGreaterThanOrEqual(48)
  expect(
    output.labels.some((label) => label.value === '--' && label.size >= 48)
  ).toBe(true)
  expect(output.labels.map((label) => label.value)).toContain('No samples')
})

test('removes generated attribution and image page numbers while preserving the custom footer', () => {
  const output = draw(
    { enabled: true, groups: [{ name: 'Pool', items: [item] }] },
    { footer: 'Custom notice' }
  )
  const values = output.labels.map((label) => label.value)
  expect(values).toContain('Custom notice')
  expect(values.join('\n')).not.toMatch(
    /QuantumNous|New API|Sub2API|\d+ \/ \d+/
  )
})

test('wraps long group and model names inside a single-column frame without clipping', () => {
  const output = draw(
    {
      enabled: true,
      groups: [
        {
          name: 'Long group '.repeat(15),
          items: [{ ...item, name: 'gpt-long-model-'.repeat(40) }],
        },
      ],
    },
    { channel_columns: 1 }
  )
  expect(output.frames).toHaveLength(1)
  const frame = output.frames[0]
  const inside = output.labels.filter(
    (label) => label.y >= frame.y && label.y < frame.y + frame.height
  )
  expect(inside.length).toBeGreaterThan(4)
  for (const label of inside) {
    expect(label.x).toBeGreaterThan(frame.x)
    expect(label.x + label.width).toBeLessThanOrEqual(
      frame.x + frame.width - 20
    )
  }
  expect(frame.y + frame.height).toBeLessThan(output.height)
})

test('hidden metrics and history are absent from the exported group contents', () => {
  const output = draw(
    { enabled: true, groups: [{ name: 'Pool', items: [item] }] },
    {
      show_success_rate: false,
      show_latency: false,
      show_history: false,
      show_updated_at: false,
    }
  )
  const values = output.labels.map((label) => label.value)
  expect(values).toContain('gpt-5')
  expect(values).toContain('Available')
  expect(values.join('\n')).not.toMatch(
    /success rate|duration|12 hours|Last sample/
  )
})

test.each([
  { enabled: false, message: 'Channel statistics disabled' },
  { enabled: true, message: 'No matching channel data' },
])(
  'empty channel report ($enabled) has an explicit message and no group frames',
  ({ enabled, message }) => {
    const output = draw({ enabled, groups: [] })
    expect(output.frames).toEqual([])
    expect(output.labels.map((label) => label.value)).toContain(message)
  }
)

const sub: SubReport = {
  status: 'connected',
  checked_at: 100,
  groups: [
    { name: 'A', ready: 1, total: 2, cooling: 0, error: 1 },
    { name: 'B', ready: 1, total: 2, cooling: 1, error: 0 },
  ],
  accounts: [
    { id: 1, name: 'OpenAI #1', groups: ['A', 'B'], status: 'normal' },
    { id: 2, name: 'OpenAI #2', groups: ['A'], status: 'failed' },
    { id: 3, name: 'OpenAI #3', groups: ['B'], status: 'cooling' },
  ],
}

test('Sub uses a black canvas and counts a cross-group account once in its primary total', () => {
  const output = draw({ enabled: true, groups: [] }, {}, sub)
  expect(output.fills[0].color).toBe('#000000')
  expect(
    output.labels.find((label) => label.value === '1 / 3')?.size
  ).toBeGreaterThanOrEqual(56)
  expect(output.frames).toHaveLength(2)
  expect(output.frames[0].y).toBe(output.frames[1].y)
})

test.each([false, true])(
  'Sub never exports account lists even when the old detail flag is $value',
  (show_sub_details) => {
    const output = draw(
      { enabled: true, groups: [] },
      { show_sub_details },
      sub
    )
    expect(output.frames).toHaveLength(2)
    const values = output.labels.map((label) => label.value)
    expect(values).toContain('1 / 3')
    expect(values.join('\n')).not.toMatch(
      /OpenAI #|Needs attention|Available accounts/
    )
  }
)

test('Sub filtering is independent from channel filtering and preserves its custom footer', () => {
  const filtered = draw(
    { enabled: true, groups: [] },
    {
      channel_group_filter: 'A',
      sub_group_filter: 'B',
      footer: 'Custom Sub notice',
    },
    sub
  )
  const values = filtered.labels.map((label) => label.value)
  expect(filtered.frames).toHaveLength(1)
  expect(values).toContain('1 / 2')
  expect(values).toContain('B')
  expect(values).not.toContain('A')
  expect(values.join('\n')).not.toContain('OpenAI #')
  expect(values).toContain('Custom Sub notice')
  expect(values.join('\n')).not.toMatch(/New API|QuantumNous/)
})

test('channel filtering ignores the selected Sub groups', () => {
  const output = draw(
    {
      enabled: true,
      groups: [
        { name: 'A', items: [item] },
        { name: 'B', items: [item] },
      ],
    },
    { channel_group_filter: 'A', sub_group_filter: 'B' }
  )
  expect(output.frames).toHaveLength(1)
  expect(output.labels.map((label) => label.value)).toContain('A')
  expect(output.labels.map((label) => label.value)).not.toContain('B')
})

test('Canvas draws only selected group-model combinations and explicit none produces an empty state', () => {
  const report = {
    enabled: true,
    groups: [
      { name: 'A', items: [item, { ...item, name: 'claude-sonnet' }] },
      { name: 'B', items: [item, { ...item, name: 'claude-sonnet' }] },
    ],
  }
  const output = draw(report, {
    channel_model_filter: [{ group: 'A', model: 'gpt-5' }],
  })
  const labels = output.labels.map((label) => label.value)
  expect(output.frames).toHaveLength(1)
  expect(labels).toContain('A')
  expect(labels).toContain('gpt-5')
  expect(labels).not.toContain('B')
  expect(labels).not.toContain('claude-sonnet')
  const empty = draw(report, { channel_model_filter: [] })
  expect(empty.frames).toEqual([])
  expect(empty.labels.map((label) => label.value)).toContain(
    'No matching channel data'
  )
})

test('explicit blank Sub filter overrides the legacy shared filter and means all groups', () => {
  const output = draw(
    { enabled: true, groups: [] },
    {
      group_filter: 'A',
      channel_group_filter: 'A',
      sub_group_filter: '',
    },
    sub
  )
  expect(output.frames).toHaveLength(2)
  expect(output.labels.map((label) => label.value)).toContain('1 / 3')
})

test('legacy shared filtering remains effective when the report-specific field is absent', () => {
  const output = draw(
    { enabled: true, groups: [] },
    {
      group_filter: 'A',
      channel_group_filter: undefined,
      sub_group_filter: undefined,
    },
    sub
  )
  expect(output.frames).toHaveLength(1)
  expect(output.labels.map((label) => label.value)).toContain('1 / 2')
})

test('explicit blank channel filter overrides the legacy shared selection independently', () => {
  const output = draw(
    {
      enabled: true,
      groups: [
        { name: 'A', items: [item] },
        { name: 'B', items: [item] },
      ],
    },
    { group_filter: 'A', channel_group_filter: '', sub_group_filter: 'A' }
  )
  expect(output.frames).toHaveLength(2)
  expect(output.labels.map((label) => label.value)).toContain('B')
})

test('disconnected Sub data never presents retained accounts as an available live pool', () => {
  const output = draw(
    { enabled: true, groups: [] },
    {},
    { ...sub, status: 'failed', error: 'Historical connection error' }
  )
  expect(output.frames).toEqual([])
  expect(output.labels.map((label) => label.value)).toContain(
    'Failed to load Sub account status'
  )
  expect(output.labels.map((label) => label.value)).not.toContain('1 / 3')
})

test('empty Sub selection shows an explicit message instead of a fabricated availability count', () => {
  const output = draw(
    { enabled: true, groups: [] },
    { sub_group_filter: 'missing' },
    sub
  )
  expect(output.frames).toEqual([])
  expect(output.labels.map((label) => label.value)).toContain(
    'No matching account data'
  )
})

test('unknown Sub status remains distinct from schedulable and failed accounts', () => {
  const output = draw(
    { enabled: true, groups: [] },
    {},
    {
      ...sub,
      groups: [{ name: 'Pool', total: 1, ready: 0, cooling: 0, error: 0 }],
      accounts: [
        { id: 1, name: 'Unknown account', groups: ['Pool'], status: 'unknown' },
      ],
    }
  )
  expect(output.labels.map((label) => label.value)).toContain('Unknown')
  expect(
    output.labels.find((label) => label.value === '0 / 1')?.size
  ).toBeGreaterThanOrEqual(56)
  expect(output.labels.map((label) => label.value)).not.toContain(
    'Available accounts'
  )
})

test('long Sub group names stay inside measured cards while account names remain omitted', () => {
  const name = 'Long selected group '.repeat(14)
  const output = draw(
    { enabled: true, groups: [] },
    {},
    {
      ...sub,
      groups: [{ ...sub.groups[0], name }],
      accounts: sub.accounts.slice(0, 2).map((account) => ({
        ...account,
        name: account.name + ' very long account name'.repeat(12),
        groups: [name],
      })),
    }
  )
  expect(output.frames).toHaveLength(1)
  for (const label of output.labels) {
    expect(label.x + label.width).toBeLessThanOrEqual(output.width - 56)
    expect(label.y + label.size).toBeLessThan(output.height)
  }
  expect(output.labels.map((label) => label.value).join('\n')).not.toMatch(
    /OpenAI #|very long account name/
  )
})
