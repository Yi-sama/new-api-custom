export function getAgnaiUrl(origin: string): string {
  const url = new URL(origin)
  if (!['localhost', '127.0.0.1'].includes(url.hostname)) {
    throw new Error('AgnAI is only configured for local testing')
  }
  url.protocol = 'http:'
  url.port = '3200'
  url.pathname = '/character/list'
  url.search = '?connect=1'
  url.hash = ''
  return url.toString()
}
