import type {
  ImageGenerationConfig,
  ImageGenerationRequest,
} from '../types'

const MAX_IMAGE_N = 128

export function buildImageGenerationPayload(
  config: ImageGenerationConfig,
  prompt: string,
  model: string,
  group: string
): ImageGenerationRequest {
  const payload: ImageGenerationRequest = {
    model,
    group,
    prompt,
    n: Math.max(1, Math.min(MAX_IMAGE_N, Math.trunc(config.n) || 1)),
    response_format: 'url',
  }

  for (const key of ['size', 'quality', 'style'] as const) {
    const value = config[key]
    if (value && value !== 'default') {
      payload[key] = value
    }
  }

  return payload
}
