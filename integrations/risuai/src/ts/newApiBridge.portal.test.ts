import { afterEach, describe, expect, it, vi } from 'vitest'
import { getNewApiPortalUrl } from './newApiBridge'

afterEach(() => vi.unstubAllEnvs())

describe('New API portal links', () => {
    it('returns to Tavern after signing in at the configured portal', () => {
        vi.stubEnv('VITE_NEW_API_PORTAL_URL', 'http://156.238.252.115:32795')

        const target = new URL(getNewApiPortalUrl('/sign-in'))

        expect(target.origin).toBe('http://156.238.252.115:32795')
        expect(target.pathname).toBe('/sign-in')
        expect(target.searchParams.get('redirect')).toBe('/tavern')
    })

    it('preserves the Tavern return path when using the relative fallback', () => {
        vi.stubEnv('VITE_NEW_API_PORTAL_URL', '')

        const target = new URL(getNewApiPortalUrl('/sign-in'), 'http://localhost:3200')

        expect(target.pathname).toBe('/sign-in')
        expect(target.searchParams.get('redirect')).toBe('/tavern')
    })

    it('does not redirect wallet or account links back to Tavern', () => {
        vi.stubEnv('VITE_NEW_API_PORTAL_URL', 'http://156.238.252.115:32795')

        expect(getNewApiPortalUrl('/wallet')).toBe('http://156.238.252.115:32795/wallet')
        expect(getNewApiPortalUrl('/dashboard')).toBe('http://156.238.252.115:32795/dashboard')
    })
})
