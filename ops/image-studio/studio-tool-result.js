function stringify(value) {
  if (value == null) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function parse(value) {
  if (typeof value !== 'string') return value
  let parsed = value.trim()
  while (typeof parsed === 'string') {
    try {
      parsed = JSON.parse(parsed)
    } catch {
      break
    }
  }
  return parsed
}

export function formatToolArguments(name, argumentsValue) {
  if (name !== 'generate_image') return stringify(argumentsValue)

  const parsedArguments = parse(argumentsValue)
  if (!parsedArguments || typeof parsedArguments !== 'object' || Array.isArray(parsedArguments)) {
    return stringify(argumentsValue)
  }

  const display = { ...parsedArguments }
  for (const key of ['size', 'aspect_ratio']) {
    if (display[key] == null || display[key] === '') delete display[key]
  }

  return stringify(display)
}

export function formatToolOutput(name, outputText) {
  if (name !== 'generate_image') return outputText

  const output = parse(outputText)
  const request = output && typeof output === 'object' && !Array.isArray(output)
    ? output.image_request
    : null
  if (!request || typeof request !== 'object' || Array.isArray(request)) return outputText

  const details = [request.model, request.size].filter(value => value != null && value !== '')
  return details.length ? `Actual request: ${details.join(' | ')}\n\n${outputText}` : outputText
}
