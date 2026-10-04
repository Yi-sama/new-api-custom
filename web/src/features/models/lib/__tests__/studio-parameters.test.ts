import { describe, expect, it } from 'vitest'

import {
  buildStudioParameters,
  readStudioParameterFormValues,
} from '../studio-parameters'

describe('studio model parameter editing', () => {
  it('preserves configured image, video and per-group options through editing', () => {
    const profile = {
      image: { sizes: ['1024x1024'], counts: [1, 2], qualities: ['standard'] },
      video: {
        durations: [4, 8],
        resolutions: ['4k'],
        modes: ['pro'],
        generate_audio: [true, false],
        watermarks: [false],
        fps: [24],
      },
      groups: { premium: { video: { durations: [8], watermarks: [false] } } },
    }
    expect(
      JSON.parse(
        buildStudioParameters(
          readStudioParameterFormValues(JSON.stringify(profile))
        )
      )
    ).toEqual(profile)
  })

  it('keeps absent parameter metadata absent rather than inserting model guesses', () => {
    expect(buildStudioParameters(readStudioParameterFormValues())).toBe('')
  })

  it('rejects malformed group and numeric options before saving', () => {
    const values = readStudioParameterFormValues()
    expect(() =>
      buildStudioParameters({ ...values, group_parameter_overrides: '[]' })
    ).toThrow()
    expect(() =>
      buildStudioParameters({ ...values, video_durations: '8, nope' })
    ).toThrow()
    expect(() =>
      buildStudioParameters({ ...values, video_generate_audio: 'yes' })
    ).toThrow()
  })
})
