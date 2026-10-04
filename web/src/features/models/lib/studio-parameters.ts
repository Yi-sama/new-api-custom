type ParameterValues = Record<string, unknown>

const fields = {
  image_sizes: ['image', 'sizes'],
  image_aspect_ratios: ['image', 'aspect_ratios'],
  image_qualities: ['image', 'qualities'],
  image_counts: ['image', 'counts'],
  video_sizes: ['video', 'sizes'],
  video_aspect_ratios: ['video', 'aspect_ratios'],
  video_durations: ['video', 'durations'],
  video_resolutions: ['video', 'resolutions'],
  video_modes: ['video', 'modes'],
  video_generate_audio: ['video', 'generate_audio'],
  video_watermarks: ['video', 'watermarks'],
  video_fps: ['video', 'fps'],
} as const

export type StudioParameterFormValues = Record<keyof typeof fields, string> & {
  group_parameter_overrides: string
}

export function readStudioParameterFormValues(
  raw?: string
): StudioParameterFormValues {
  const parsed = raw ? (JSON.parse(raw) as Record<string, ParameterValues>) : {}
  const result = {} as StudioParameterFormValues
  for (const field of Object.keys(fields) as Array<keyof typeof fields>) {
    const [kind, key] = fields[field]
    let values = parsed[kind]?.[key]
    if (
      Array.isArray(values) &&
      (key === 'generate_audio' || key === 'watermarks')
    ) {
      values = [...values].sort((a, b) => Number(b) - Number(a))
    }
    result[field] = Array.isArray(values) ? values.join(', ') : ''
  }
  result.group_parameter_overrides = parsed.groups
    ? JSON.stringify(parsed.groups, null, 2)
    : ''
  return result
}

export function buildStudioParameters(
  values: StudioParameterFormValues
): string {
  const profile: Record<string, ParameterValues> = {}
  for (const field of Object.keys(fields) as Array<keyof typeof fields>) {
    const [kind, key] = fields[field]
    const entries = values[field]
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
    if (!entries.length) continue
    let options: Array<string | number | boolean> = entries
    if (key === 'generate_audio' || key === 'watermarks') {
      if (entries.some((item) => item !== 'true' && item !== 'false')) {
        throw new Error('Invalid boolean option')
      }
      options = entries.map((item) => item === 'true')
    } else if (key === 'counts' || key === 'durations' || key === 'fps') {
      options = entries.map(Number)
      if (
        options.some((item) => !Number.isSafeInteger(item) || Number(item) <= 0)
      ) {
        throw new Error('Invalid numeric option')
      }
    }
    profile[kind] ??= {}
    profile[kind][key] = options
  }
  if (values.group_parameter_overrides.trim()) {
    const groups: unknown = JSON.parse(values.group_parameter_overrides)
    if (!groups || typeof groups !== 'object' || Array.isArray(groups)) {
      throw new Error('Group profiles must be an object')
    }
    if (Object.keys(groups).length) profile.groups = groups as ParameterValues
  }
  return Object.keys(profile).length ? JSON.stringify(profile) : ''
}
