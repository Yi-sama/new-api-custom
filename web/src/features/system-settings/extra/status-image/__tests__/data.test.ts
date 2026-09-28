import { expect, test } from 'vitest'

import { defaultStatusImageConfig, statusImageSchema } from '../config'
import { channelReport } from '../data'

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
