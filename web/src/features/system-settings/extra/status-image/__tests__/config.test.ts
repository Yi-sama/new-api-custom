import { expect, test } from 'vitest'

import {
  defaultStatusImageConfig,
  statusImageGroupFilter,
  statusImageSchema,
  type StatusImageConfig,
} from '../config'

test.each(['channel', 'sub'] as const)(
  '%s falls back only when its independent selection is absent, not blank',
  (kind) => {
    const config: StatusImageConfig = {
      ...defaultStatusImageConfig,
      group_filter: 'Legacy',
      channel_group_filter: undefined,
      sub_group_filter: undefined,
    }
    expect(statusImageGroupFilter(statusImageSchema.parse(config), kind)).toBe(
      'Legacy'
    )
    config[`${kind}_group_filter`] = ''
    expect(statusImageGroupFilter(statusImageSchema.parse(config), kind)).toBe(
      ''
    )
  }
)

test.each(['channel_group_filter', 'sub_group_filter'] as const)(
  '%s accepts 500 characters and rejects oversized or non-string selections',
  (field) => {
    for (const value of [undefined, '', 'x'.repeat(500)]) {
      expect(
        statusImageSchema.safeParse({
          ...defaultStatusImageConfig,
          [field]: value,
        }).success
      ).toBe(true)
    }
    for (const value of ['x'.repeat(501), 1, ['Pool']]) {
      expect(
        statusImageSchema.safeParse({
          ...defaultStatusImageConfig,
          [field]: value,
        }).success
      ).toBe(false)
    }
  }
)

test('model selection accepts legacy all, explicit none and exact pairs but rejects malformed selections', () => {
  for (const value of [
    undefined,
    null,
    [],
    [{ group: 'Pool', model: 'gpt-5' }],
  ]) {
    const parsed = statusImageSchema.parse({
      ...defaultStatusImageConfig,
      channel_model_filter: value,
    })
    expect(parsed.channel_model_filter).toEqual(value)
  }
  for (const value of [
    '',
    ['gpt-5'],
    [{}],
    [{ group: ' ', model: 'gpt-5' }],
    [{ group: 'Pool', model: null }],
    [{ group: 'Pool', model: 'x'.repeat(501) }],
    Array.from({ length: 501 }, () => ({ group: 'Pool', model: 'gpt-5' })),
  ]) {
    expect(
      statusImageSchema.safeParse({
        ...defaultStatusImageConfig,
        channel_model_filter: value,
      }).success
    ).toBe(false)
  }
})
