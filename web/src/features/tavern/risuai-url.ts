export function getRisuaiUrl(
  origin: string,
  configuredUrl = process.env.VITE_RISUAI_URL
): string {
  const portalUrl = new URL(origin)
  if (configuredUrl) {
    const risuaiUrl = new URL(configuredUrl)
    if (!['http:', 'https:'].includes(risuaiUrl.protocol)) {
      throw new Error('RisuAI URL must use HTTP or HTTPS')
    }
    if (risuaiUrl.username || risuaiUrl.password) {
      throw new Error('RisuAI URL must not include userinfo')
    }
    if (risuaiUrl.hostname !== portalUrl.hostname) {
      throw new Error('RisuAI URL must use the portal hostname')
    }
    return risuaiUrl.toString()
  }

  if (!['localhost', '127.0.0.1'].includes(portalUrl.hostname)) {
    throw new Error('RisuAI is only configured for local testing')
  }
  portalUrl.protocol = 'http:'
  portalUrl.port = '3200'
  portalUrl.pathname = '/'
  portalUrl.search = '?connect=1'
  portalUrl.hash = ''
  return portalUrl.toString()
}
