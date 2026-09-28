import { describe, expect, it } from 'vitest'

import { buildImageGenerationPayload } from '../image-generation-payload'
import type { ImageGenerationConfig } from '../../types'

const baseConfig: ImageGenerationConfig = {
  n: 2,
  size: '1024x1024',
  quality: 'high',
  style: 'vivid',
}

describe('buildImageGenerationPayload', () => {
  it('includes the supported image fields', () => {
    const payload = buildImageGenerationPayload(
      baseConfig,
      'a red fox in a library',
      'gpt-image-1',
      'default'
    )

    expect(payload).toMatchObject({
      model: 'gpt-image-1',
      group: 'default',
      prompt: 'a red fox in a library',
      n: 2,
      response_format: 'url',
      size: '1024x1024',
      quality: 'high',
      style: 'vivid',
    })
  })

  it('omits default values and clamps n to the backend limit', () => {
    const payload = buildImageGenerationPayload(
      {
        ...baseConfig,
        n: 999,
        size: 'default',
        quality: 'default',
        style: 'default',
      },
      'a quiet mountain lake',
      'dall-e-3',
      'vip'
    )

    expect(payload).toMatchObject({
      n: 128,
      group: 'vip',
      response_format: 'url',
    })
    expect(payload).not.toHaveProperty('size')
    expect(payload).not.toHaveProperty('quality')
    expect(payload).not.toHaveProperty('style')
  })

  it('uses the URL response format even when defaults are selected', () => {
    const payload = buildImageGenerationPayload(
      {
        n: 1,
        size: 'default',
        quality: 'default',
        style: 'default',
      },
      'a small paper boat',
      'gpt-image-1',
      ''
    )

    expect(payload).toEqual({
      model: 'gpt-image-1',
      group: '',
      prompt: 'a small paper boat',
      n: 1,
      response_format: 'url',
    })
  })
})
