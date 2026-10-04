import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LLMFormat } from './model/types'

let bridge: typeof import('./newApiBridge')
const fetchMock = vi.fn()

function jsonResponse(data: unknown, status = 200) {
    return new Response(JSON.stringify(data), {
        status,
        headers: { 'content-type': 'application/json' }
    })
}

beforeAll(async () => {
    vi.stubEnv('VITE_NEW_API_BRIDGE', '1')
    vi.stubGlobal('fetch', fetchMock)
    bridge = await import('./newApiBridge')
})

beforeEach(() => {
    fetchMock.mockReset()
    localStorage.clear()
})

afterAll(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
})

describe('New API bridge', () => {
    it('authenticates before exposing pinned protected-request headers', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({
            userId: 'user-1',
            sid: 'session-1',
            accessExpiresAt: Math.floor(Date.now() / 1_000) + 300,
            portalUrl: 'https://portal.example.test'
        }))

        await bridge.authenticateNewApi()
        const headers = await bridge.getNewApiHeaders({ 'content-type': 'application/json' })

        expect(fetchMock).toHaveBeenCalledWith('/api/user/auth/new-api', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ reconnect: false })
        }))
        expect(headers).toMatchObject({
            'content-type': 'application/json',
            'X-New-API-User': 'user-1',
            'X-New-API-Session': 'session-1'
        })
        expect(headers.Authorization).toBeUndefined()
        expect(fetchMock).toHaveBeenCalledTimes(1)
    })

    it('uses the selected group and model for the native OpenAI pipeline', () => {
        const db = {
            aiModel: 'gpt-4',
            subModel: 'claude',
            forceReplaceUrl: '',
            proxyKey: 'should-not-survive',
            customProxyRequestModel: '',
            customAPIFormat: 18 as LLMFormat,
            autofillRequestUrl: true,
            seperateModelsForAxModels: true,
            customTokenizer: 'claude',
            reverseProxyOobaMode: true,
            enableCustomFlags: true,
            sdProvider: 'webui',
            hypaModel: 'custom',
            supaModelType: 'instruct35',
            translatorType: 'deeplX',
            requestLocation: 'user-proxy',
            localNetworkMode: true,
            usePlainFetch: true,
            enableRemoteSaving: true,
            forceProxyAsOpenAI: true,
            openAIKey: 'keep-imported-secret',
        }

        bridge.applyNewApiModelSelection(db, 'premium', 'model-x')

        expect(db).toMatchObject({
            aiModel: 'reverse_proxy',
            subModel: 'reverse_proxy',
            forceReplaceUrl: '/api/new-api/chat/completions',
            proxyKey: '',
            customProxyRequestModel: 'model-x',
            customAPIFormat: 0,
            autofillRequestUrl: false,
            seperateModelsForAxModels: false,
            customTokenizer: 'tiktoken',
            reverseProxyOobaMode: false,
            enableCustomFlags: false,
            sdProvider: '',
            hypaModel: 'MiniLM',
            supaModelType: 'subModel',
            translatorType: 'bergamot',
            requestLocation: '',
            localNetworkMode: false,
            usePlainFetch: false,
            enableRemoteSaving: false,
            forceProxyAsOpenAI: false,
            openAIKey: 'keep-imported-secret',
        })
        expect(bridge.isNewApiChatUrl('/api/new-api/chat/completions')).toBe(true)
        expect(bridge.isNewApiChatUrl('https://example.test/api/new-api/chat/completions')).toBe(false)
    })

    it('clears stale model selections when no chat model is available', () => {
        const db = {
            aiModel: '', subModel: '', forceReplaceUrl: '', proxyKey: '', customProxyRequestModel: '',
            customAPIFormat: 0 as LLMFormat, autofillRequestUrl: false, seperateModelsForAxModels: false
        }
        bridge.applyNewApiModelSelection(db, 'images', 'Auto')
        bridge.applyNewApiModelSelection(db, '', '')
        expect(bridge.getNewApiSelection()).toEqual({ group: '', model: '' })
    })

    it('keeps response metadata pinned to the model selected at dispatch', () => {
        const modelAtDispatch = 'fixture-chat'
        localStorage.setItem('new-api.model', 'changed-after-dispatch')

        expect(bridge.getNewApiResponseModel(modelAtDispatch, '4.6 Sonnet', 'provider-model')).toBe('fixture-chat')
    })

    it('keeps outgoing group and model headers pinned to dispatch selection', async () => {
        const headers = await bridge.getNewApiChatHeaders({}, {
            group: 'premium',
            model: 'fixture-premium'
        })

        expect(headers).toMatchObject({
            'X-New-API-Group': 'premium',
            'X-New-API-Model': 'fixture-premium'
        })
    })

    it('never upgrades a refresh into reconnect or accepts an identity change', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({
            userId: 'user-1',
            sid: 'session-1',
            accessExpiresAt: Math.floor(Date.now() / 1_000) + 10,
            portalUrl: 'https://portal.example.test'
        }))
        await bridge.authenticateNewApi()

        fetchMock.mockResolvedValueOnce(jsonResponse({
            userId: 'user-2',
            sid: 'session-2',
            accessExpiresAt: Math.floor(Date.now() / 1_000) + 300,
            portalUrl: 'https://portal.example.test'
        }))

        await expect(bridge.refreshNewApiSession()).rejects.toThrow('identity changed')
        expect(fetchMock).toHaveBeenLastCalledWith('/api/user/auth/new-api', expect.objectContaining({
            body: JSON.stringify({ reconnect: false })
        }))
        await expect(bridge.getNewApiHeaders()).rejects.toThrow('identity is frozen')
    })

    it('formats quota displays using the service conversion fields', () => {
        const account = {
            account: { id: '1', username: 'user', display_name: 'User', quota: 2_500_000, group: 'default' },
            groups: {},
            models: [],
            group: 'default',
            quotaPerUnit: 500_000,
            quotaDisplayType: 'USD',
            usdExchangeRate: 7.2,
            customCurrencySymbol: 'C',
            customCurrencyExchangeRate: 3,
        }

        expect(bridge.formatNewApiBalance(account)).toBe('$5')
        expect(bridge.formatNewApiBalance({ ...account, quotaDisplayType: 'CNY' })).toBe('¥36')
        expect(bridge.formatNewApiBalance({ ...account, quotaDisplayType: 'CUSTOM' })).toBe('C 15')
        expect(bridge.formatNewApiBalance({ ...account, quotaDisplayType: 'TOKENS' })).toBe('2500k')
        const withGift = { ...account, tavernCredit: { quota: 123456, granted_quota: 500000, created_time: 1 } }
        expect(bridge.formatNewApiBalance(withGift)).toBe('$5')
        expect(bridge.formatNewApiTavernCredit(withGift)).toBe('$0.246912')
        expect(bridge.formatNewApiTavernCredit(withGift, true)).toBe('$1.00')
        expect(bridge.formatNewApiTavernCredit({ ...withGift, quotaDisplayType: 'CNY' }, true)).toBe('$1.00')
    })
})
