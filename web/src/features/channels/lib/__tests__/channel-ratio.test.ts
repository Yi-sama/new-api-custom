import { describe, expect, it } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  transformChannelToFormDefaults,
} from '../channel-form'

describe('channel ratio', () => {
  it.each([null, 0, 0.15])(
    'preserves %s in create and update payloads',
    (ratio) => {
      const values = { ...CHANNEL_FORM_DEFAULT_VALUES, channel_ratio: ratio }
      expect(
        transformFormDataToCreatePayload(values).channel.channel_ratio
      ).toBe(ratio)
      expect(transformFormDataToUpdatePayload(values, 1).channel_ratio).toBe(
        ratio
      )
    }
  )
  it('rejects negative and non-finite ratios', () => {
    const values = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'A',
      key: 'test',
      models: 'gpt-4o',
    }
    for (const ratio of [-1, Infinity, Number.NaN]) {
      expect(
        channelFormSchema.safeParse({ ...values, channel_ratio: ratio }).success
      ).toBe(false)
    }
  })
  it('keeps zero when loading a channel', () => {
    const channel = channelSchema.parse({
      id: 1,
      type: 1,
      key: '',
      name: 'A',
      status: 1,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      channel_ratio: 0,
    })
    expect(transformChannelToFormDefaults(channel).channel_ratio).toBe(0)
  })
})
