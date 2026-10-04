import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Tavern } from '../index'
import { getRisuaiUrl } from '../risuai-url'

afterEach(() => vi.unstubAllEnvs())

describe('RisuAI standalone entry', () => {
  it('opens the actual upstream app without an iframe or custom chat', async () => {
    const navigate = vi.fn()
    render(<Tavern navigate={navigate} />)
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith('http://localhost:3200/?connect=1')
    )
    expect(screen.getByRole('heading', { name: 'RisuAI' })).toBeInTheDocument()
    expect(document.querySelector('iframe')).toBeNull()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('keeps the hostname to share the existing local login cookie', () => {
    expect(getRisuaiUrl('http://127.0.0.1:3107')).toBe(
      'http://127.0.0.1:3200/?connect=1'
    )
  })

  it('uses an opt-in deployed URL on the same portal hostname', () => {
    expect(
      getRisuaiUrl(
        'http://156.238.252.115:32795',
        'http://156.238.252.115:32796/?connect=1'
      )
    ).toBe('http://156.238.252.115:32796/?connect=1')
  })

  it('reads the deployed URL from VITE_RISUAI_URL', () => {
    vi.stubEnv('VITE_RISUAI_URL', 'http://156.238.252.115:32796/?connect=1')

    expect(getRisuaiUrl('http://156.238.252.115:32795')).toBe(
      'http://156.238.252.115:32796/?connect=1'
    )
  })

  it('does not send production users to their own localhost', () => {
    expect(() => getRisuaiUrl('https://example.com')).toThrow(
      'RisuAI is only configured for local testing'
    )
  })

  it.each([
    ['a non-HTTP protocol', 'javascript:alert(1)', 'HTTP or HTTPS'],
    [
      'userinfo',
      'https://user:pass@example.com/?connect=1',
      'must not include userinfo',
    ],
    [
      'a different hostname',
      'https://risu.example.com/?connect=1',
      'must use the portal hostname',
    ],
  ])('rejects %s in the configured URL', (_case, configuredUrl, message) => {
    expect(() => getRisuaiUrl('https://example.com', configuredUrl)).toThrow(
      message
    )
  })
})
