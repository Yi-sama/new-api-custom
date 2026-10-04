import { afterAll, beforeEach, expect, it, vi } from 'vitest'
import type { character } from '../storage/database.svelte'

const { alertError, requestChatData, globalFetch } = vi.hoisted(() => ({
    alertError: vi.fn(),
    requestChatData: vi.fn(),
    globalFetch: vi.fn(),
}))

vi.mock('../newApiBridge', () => ({ isNewApiBridge: () => true }))
vi.mock('../alert', () => ({ alertError }))
vi.mock('../storage/database.svelte', () => ({
    getDatabase: () => ({ sdProvider: 'webui', elevenLabKey: 'legacy-key' }),
    getCurrentCharacter: vi.fn(),
}))
vi.mock('../globalApi.svelte', () => ({ globalFetch, fetchNative: vi.fn(), readImage: vi.fn(), loadAsset: vi.fn() }))
vi.mock('./request/request', () => ({ requestChatData }))
vi.mock('../stores.svelte', () => ({ CharEmotion: {} }))
vi.mock('../util', () => ({ sleep: vi.fn() }))
vi.mock('./processzip', () => ({ processZip: vi.fn() }))
vi.mock('../kei/kei', () => ({ keiServerURL: '' }))
vi.mock('../translator/translator', () => ({ runTranslator: vi.fn(), translateVox: vi.fn() }))
vi.mock('./transformers', () => ({ runVITS: vi.fn() }))
vi.mock('src/lang', () => ({ language: { setup: { newApiServiceUnavailable: 'Service unavailable' } } }))

import { generateAIImage, stableDiff } from './stableDiff'
import { sayTTS } from './tts'

beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('fetch', vi.fn())
})
afterAll(() => vi.unstubAllGlobals())

it('blocks legacy image providers before prompt generation or network access', async () => {
    const char = { type: 'character' } as character
    expect(await stableDiff(char, 'image prompt')).toBe(false)
    expect(await generateAIImage('image prompt', char, '', '')).toBe(false)
    expect(requestChatData).not.toHaveBeenCalled()
    expect(globalFetch).not.toHaveBeenCalled()
    expect(fetch).not.toHaveBeenCalled()
    expect(alertError).toHaveBeenCalledWith('Service unavailable')
})

it('blocks imported remote TTS credentials before network access', async () => {
    await sayTTS({ type: 'character', ttsMode: 'elevenlab' } as character, 'hello')
    expect(fetch).not.toHaveBeenCalled()
    expect(globalFetch).not.toHaveBeenCalled()
    expect(alertError).toHaveBeenCalledWith('Service unavailable')
})
