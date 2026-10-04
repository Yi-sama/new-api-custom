import { afterEach, describe, expect, it, vi } from 'vitest'

describe('New API refresh failure handling', () => {
    afterEach(() => {
        vi.unstubAllEnvs()
        vi.unstubAllGlobals()
    })

    it('freezes the active identity when a refresh fails', async () => {
        vi.resetModules()
        vi.stubEnv('VITE_NEW_API_BRIDGE', '1')
        const fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)
        const bridge = await import('./newApiBridge')

        fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({
            userId: 'user-1',
            sid: 'session-1',
            accessExpiresAt: Math.floor(Date.now() / 1_000) + 300,
            portalUrl: 'https://portal.example.test'
        }), { status: 200 }))
        await bridge.authenticateNewApi()

        fetchMock.mockResolvedValueOnce(new Response(null, { status: 401 }))
        await expect(bridge.refreshNewApiSession()).rejects.toThrow('401')
        await expect(bridge.getNewApiHeaders()).rejects.toThrow('identity is frozen')
    })
})
