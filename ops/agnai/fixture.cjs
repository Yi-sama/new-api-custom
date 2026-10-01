// Isolated New API protocol fixture. Never point this at a real provider or database.
const http = require('node:http')

const users = new Map([
  ['901', { id: 901, username: 'fixture_alice', display_name: 'TEST FIXTURE / Alice', role: 1, status: 1, group: 'default', quota: 5000000 }],
  ['902', { id: 902, username: 'fixture_bob', display_name: 'TEST FIXTURE / Bob', role: 1, status: 1, group: 'default', quota: 9000000 }],
])
const revoked = new Set()
const requests = []

function json(res, code, body) {
  res.writeHead(code, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
  res.end(JSON.stringify(body))
}

http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1:3211')
  if (url.pathname === '/') {
    res.writeHead(200, { 'Content-Type': 'text/html' })
    return res.end('<h1>Isolated AgnAI integration fixture</h1><p>All balances and replies here are synthetic.</p><a href="/test/connect?user=901">Test Alice</a><br><a href="/test/connect?user=902">Test Bob</a>')
  }
  if (url.pathname === '/test/connect') {
    const id = url.searchParams.get('user')
    if (!users.has(id)) return json(res, 404, {})
    revoked.delete(id)
    res.writeHead(302, {
      'Set-Cookie': `new_api_refresh=fixture-${id}; HttpOnly; SameSite=Strict; Path=/api/user/auth`,
      Location: 'http://127.0.0.1:3210/character/list?connect=1',
    })
    return res.end()
  }
  if (url.pathname === '/test/state') {
    return json(res, 200, { users: [...users.values()], requests })
  }
  if (url.pathname === '/test/revoke' && req.method === 'POST') {
    revoked.add(url.searchParams.get('user'))
    return json(res, 200, { success: true })
  }
  if (url.pathname === '/test/recharge' && req.method === 'POST') {
    const user = users.get(url.searchParams.get('user'))
    if (user) user.quota += 500000
    return json(res, 200, { success: !!user })
  }
  if (url.pathname === '/api/status') {
    return json(res, 200, { success: true, data: { quota_per_unit: 500000, quota_display_type: 'USD', usd_exchange_rate: 7 } })
  }
  if (url.pathname === '/api/user/auth/refresh' || url.pathname === '/api/user/auth/logout') {
    const match = /(?:^|;\s*)new_api_refresh=fixture-(901|902)(?:;|$)/.exec(req.headers.cookie || '')
    const id = match?.[1]
    if (!id || revoked.has(id)) return json(res, 401, { success: false, message: 'Fixture sign-in required' })
    const sid = `fixture-sid-${id}`
    if (req.headers['x-auth-session'] && req.headers['x-auth-session'] !== sid) {
      return json(res, 409, { success: false, message: 'Fixture account changed' })
    }
    if (url.pathname.endsWith('/logout')) {
      revoked.add(id)
      res.setHeader('Set-Cookie', 'new_api_refresh=; HttpOnly; SameSite=Strict; Path=/api/user/auth; Max-Age=0')
      return json(res, 200, { success: true })
    }
    res.setHeader('Set-Cookie', `new_api_refresh=fixture-${id}; HttpOnly; SameSite=Strict; Path=/api/user/auth`)
    return json(res, 200, { success: true, data: {
      access_token: `fixture-access-${id}`, token_type: 'Bearer',
      access_expires_at: Math.floor(Date.now() / 1000) + 900,
      user: users.get(id), session: { sid, current: true, expires_at: Math.floor(Date.now() / 1000) + 86400 },
    } })
  }
  const id = /^Bearer fixture-access-(901|902)$/.exec(req.headers.authorization || '')?.[1]
  const user = users.get(id)
  if (!user || revoked.has(id)) return json(res, 401, { success: false, message: 'Fixture session revoked' })
  if (url.pathname === '/api/user/self') return json(res, 200, { success: true, data: user })
  if (url.pathname === '/api/user/self/groups') {
    return json(res, 200, { success: true, data: { default: { desc: 'Fixture standard', ratio: 1 }, premium: { desc: 'Fixture premium', ratio: 2 } } })
  }
  if (url.pathname === '/api/user/models') {
    const group = url.searchParams.get('group')
    if (!['default', 'premium'].includes(group)) return json(res, 403, { success: false })
    return json(res, 200, { success: true, data: group === 'premium' ? ['fixture-premium'] : ['fixture-chat', 'fixture-chat-2'] })
  }
  if (url.pathname === '/pg/chat/completions' && req.method === 'POST') {
    let raw = ''
    for await (const chunk of req) raw += chunk
    const body = JSON.parse(raw)
    if (!['default', 'premium'].includes(body.group) || !body.model.startsWith('fixture-')) {
      return json(res, 403, { error: { message: 'Fixture denied model/group' } })
    }
    if (user.quota < 12500) return json(res, 403, { error: { message: 'Fixture insufficient balance' } })
    user.quota -= 12500
    requests.push({ userId: user.id, model: body.model, group: body.group, stream: body.stream, messages: body.messages })
    const reply = 'This is an isolated AgnAI integration test reply. No paid model was called.'
    if (!body.stream) return json(res, 200, {
      id: 'fixture-chat', object: 'chat.completion', model: body.model,
      choices: [{ index: 0, finish_reason: 'stop', message: { role: 'assistant', content: reply } }],
      usage: { prompt_tokens: 10, completion_tokens: 15, total_tokens: 25 },
    })
    res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-store' })
    res.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { role: 'assistant', content: reply }, finish_reason: null }] })}\n\n`)
    res.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: {}, finish_reason: 'stop' }], usage: { prompt_tokens: 10, completion_tokens: 15, total_tokens: 25 } })}\n\n`)
    return res.end('data: [DONE]\n\n')
  }
  json(res, 404, { success: false, message: 'Unknown fixture endpoint' })
}).listen(3211, '127.0.0.1', () => console.log('Synthetic fixture only: http://127.0.0.1:3211'))
