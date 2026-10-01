import { describe, expect, it } from 'vitest'

import config from '../../../../rsbuild.config'

describe('Video Studio development routing', () => {
  it('forwards Canvas documents and assets to the same backend as the API', async () => {
    const resolved = await config({
      envMode: 'development',
      env: 'development',
      command: 'dev',
    })
    expect(resolved.server?.proxy).toHaveProperty('/video-canvas')
    expect(resolved.server?.proxy).toHaveProperty(
      '/video-canvas',
      expect.objectContaining({
        target: expect.any(String),
        changeOrigin: true,
      })
    )
    const proxy = resolved.server?.proxy as Record<string, unknown>
    expect(proxy['/video-canvas']).toEqual(proxy['/api'])
  })
})
