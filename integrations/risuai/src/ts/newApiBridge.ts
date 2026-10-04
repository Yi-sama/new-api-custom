import type { LLMFormat } from "./model/types"

export interface NewApiIdentity {
    userId: string
    sid: string
    accessExpiresAt: number
    portalUrl: string
}

export interface NewApiAccountResponse {
    systemName?: string
    tavernCredit?: {
        quota: number
        granted_quota: number
        created_time: number
    }
    account: {
        id: string
        username: string
        display_name: string
        quota: number
        group: string
    }
    groups: Record<string, { desc: string, ratio: number }>
    models: string[]
    group: string
    quotaPerUnit: number
    quotaDisplayType: string
    usdExchangeRate: number
    customCurrencySymbol: string
    customCurrencyExchangeRate: number
}

export interface NewApiChatSelection {
    group: string
    model: string
}

export function formatNewApiTavernCredit(account: NewApiAccountResponse, granted = false): string {
    const quota = (granted ? account.tavernCredit?.granted_quota : account.tavernCredit?.quota) || 0
    return new Intl.NumberFormat('en-US', {
        style: 'currency', currency: 'USD', maximumFractionDigits: 6,
    }).format(quota / Math.max(account.quotaPerUnit || 1, 1))
}

export function formatNewApiBalance(account: NewApiAccountResponse): string {
    const displayType = account.quotaDisplayType.toUpperCase()
    const unitQuota = account.account.quota / Math.max(account.quotaPerUnit || 1, 1)
    const formatNumber = (value: number, digitsLarge = 2, digitsSmall = 4, abbreviate = true) => {
        if(abbreviate && Math.abs(value) >= 1_000){
            return `${Number((value / 1_000).toFixed(1))}k`
        }
        return Number(value.toFixed(Math.abs(value) >= 1 ? digitsLarge : digitsSmall)).toString()
    }
    const formatCurrency = (value: number, currency: 'USD' | 'CNY') => new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency,
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: 0,
        maximumFractionDigits: Math.abs(value) >= 1 ? 2 : 4
    }).format(value)
    switch(displayType){
        case 'CNY':
            return formatCurrency(unitQuota * account.usdExchangeRate, 'CNY')
        case 'CUSTOM':
            return `${account.customCurrencySymbol || ''} ${formatNumber(unitQuota * account.customCurrencyExchangeRate)}`
        case 'TOKENS':
            return formatNumber(account.account.quota, 0)
        case 'USD':
        default:
            return formatCurrency(unitQuota, 'USD')
    }
}

const bridgeEnabled = import.meta.env.VITE_NEW_API_BRIDGE === '1'
const refreshLeewayMs = 60_000

let identity: NewApiIdentity | null = null
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let storageNamespaceInstalled = false
let localForageNamespaceInstalled = false
let storagePrefix = ''
let authRequest: Promise<NewApiIdentity> | null = null
let identityFrozen = false
let explicitReconnectConsumed = false
let systemName = ''

export function getNewApiSiteName(account?: NewApiAccountResponse | null): string {
    return account?.systemName || systemName
}

export function isNewApiBridge(): boolean {
    return bridgeEnabled
}

function validateIdentity(value: unknown): NewApiIdentity {
    const data = value as Partial<NewApiIdentity>
    if(
        !data ||
        typeof data.userId !== 'string' ||
        typeof data.sid !== 'string' ||
        typeof data.accessExpiresAt !== 'number' ||
        typeof data.portalUrl !== 'string'
    ){
        throw new Error('New API authentication returned an invalid response.')
    }
    return data as NewApiIdentity
}

function installStorageNamespace(userId: string) {
    if(storageNamespaceInstalled || typeof window === 'undefined'){
        return
    }

    storagePrefix = `risuai:new-api:${encodeURIComponent(userId)}:`
    const local = window.localStorage
    const prototype = Storage.prototype
    const getItem = prototype.getItem
    const setItem = prototype.setItem
    const removeItem = prototype.removeItem
    const clear = prototype.clear
    const key = prototype.key
    const length = Object.getOwnPropertyDescriptor(prototype, 'length')
    const scopedKeys = (storage: Storage) => {
        const keys: string[] = []
        for(let index = 0; index < (length?.get?.call(storage) ?? 0); index++){
            const rawKey = key.call(storage, index)
            if(rawKey?.startsWith(storagePrefix)){
                keys.push(rawKey.slice(storagePrefix.length))
            }
        }
        return keys
    }
    const isLocal = (storage: Storage) => storage === local

    prototype.getItem = function(this: Storage, itemKey: string) {
        return getItem.call(this, isLocal(this) ? storagePrefix + itemKey : itemKey)
    }
    prototype.setItem = function(this: Storage, itemKey: string, value: string) {
        return setItem.call(this, isLocal(this) ? storagePrefix + itemKey : itemKey, value)
    }
    prototype.removeItem = function(this: Storage, itemKey: string) {
        return removeItem.call(this, isLocal(this) ? storagePrefix + itemKey : itemKey)
    }
    prototype.clear = function(this: Storage) {
        if(!isLocal(this)){
            return clear.call(this)
        }
        for(const itemKey of scopedKeys(this)){
            removeItem.call(this, storagePrefix + itemKey)
        }
    }
    prototype.key = function(this: Storage, index: number) {
        return isLocal(this) ? (scopedKeys(this)[index] ?? null) : key.call(this, index)
    }
    if(length?.configurable){
        Object.defineProperty(prototype, 'length', {
            configurable: true,
            get(this: Storage) {
                return isLocal(this) ? scopedKeys(this).length : length.get?.call(this)
            }
        })
    }
    storageNamespaceInstalled = true
}

async function installLocalForageNamespace(userId: string) {
    if(localForageNamespaceInstalled){
        return
    }
    const { default: localforage } = await import('localforage')
    const prefix = `risuai:new-api:${encodeURIComponent(userId)}:`
    const createInstance = localforage.createInstance.bind(localforage)
    localforage.config({ name: prefix + 'default' })
    localforage.createInstance = (options = {}) => createInstance({
        ...options,
        name: prefix + (options.name || 'default')
    })
    localForageNamespaceInstalled = true
}

function expiresAtMs(value: NewApiIdentity): number {
    return value.accessExpiresAt * 1_000
}

function canUseExplicitReconnect(): boolean {
    return !identity && !explicitReconnectConsumed && new URLSearchParams(location.search).get('connect') === '1'
}

function freezeIdentity() {
    identity = null
    identityFrozen = true
    if(refreshTimer){
        clearTimeout(refreshTimer)
        refreshTimer = null
    }
}

function scheduleRefresh() {
    if(refreshTimer){
        clearTimeout(refreshTimer)
        refreshTimer = null
    }
    if(!identity){
        return
    }
    const delay = Math.max(1_000, expiresAtMs(identity) - Date.now() - refreshLeewayMs)
    refreshTimer = setTimeout(() => {
        void refreshNewApiSession().catch((error) => console.error('New API session refresh failed:', error))
    }, delay)
}

export async function authenticateNewApi(reconnect = false): Promise<NewApiIdentity> {
    if(!bridgeEnabled){
        throw new Error('New API bridge is disabled.')
    }
    if(identityFrozen){
        throw new Error('New API identity is frozen after a session mismatch. Reopen the app before continuing.')
    }
    if(authRequest){
        return await authRequest
    }
    if(reconnect && !canUseExplicitReconnect()){
        throw new Error('New API reconnect is only available on the initial ?connect=1 page load.')
    }
    const useReconnect = reconnect && canUseExplicitReconnect()
    if(useReconnect){
        explicitReconnectConsumed = true
    }
    authRequest = (async () => {
        try {
            const response = await fetch('/api/user/auth/new-api', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'content-type': 'application/json' },
                body: JSON.stringify({ reconnect: useReconnect })
            })
            if(!response.ok){
                throw new Error(`New API authentication failed (${response.status}).`)
            }
            const nextIdentity = validateIdentity(await response.json())
            if(identity && (identity.userId !== nextIdentity.userId || identity.sid !== nextIdentity.sid)){
                throw new Error('New API identity changed in this tab. Reopen the app before accessing account data.')
            }
            identity = nextIdentity
            installStorageNamespace(identity.userId)
            await installLocalForageNamespace(identity.userId)
            scheduleRefresh()
            return identity
        } catch(error) {
            if(identity){
                freezeIdentity()
            }
            throw error
        }
    })()
    try {
        return await authRequest
    } finally {
        authRequest = null
    }
}

export async function ensureNewApiSession(forceRefresh = false): Promise<NewApiIdentity> {
    if(!bridgeEnabled){
        throw new Error('New API bridge is disabled.')
    }
    if(identityFrozen){
        throw new Error('New API identity is frozen after a session mismatch. Reopen the app before continuing.')
    }
    if(!forceRefresh && identity && expiresAtMs(identity) > Date.now() + refreshLeewayMs){
        return identity
    }
    return await authenticateNewApi(false)
}

export async function refreshNewApiSession(): Promise<NewApiIdentity> {
    return await ensureNewApiSession(true)
}

export async function initializeNewApiBridge() {
    if(!bridgeEnabled){
        return
    }
    await authenticateNewApi(canUseExplicitReconnect())
    window.addEventListener('focus', () => {
        void refreshNewApiSession().catch((error) => console.error('New API session refresh failed:', error))
    })
}

export async function getNewApiHeaders(headers: Record<string, string> = {}): Promise<Record<string, string>> {
    const current = await ensureNewApiSession()
    return {
        ...headers,
        'X-New-API-User': current.userId,
        'X-New-API-Session': current.sid
    }
}

export function getNewApiGroup(): string {
    return bridgeEnabled ? (localStorage.getItem('new-api.group') ?? '') : ''
}

export function getNewApiModel(): string {
    return bridgeEnabled ? (localStorage.getItem('new-api.model') ?? '') : ''
}

export function getNewApiSelection(): NewApiChatSelection {
    return { group: getNewApiGroup(), model: getNewApiModel() }
}

export function getNewApiResponseModel(modelAtDispatch: string, fallbackModel?: string, responseModel?: string): string | undefined {
    return modelAtDispatch || fallbackModel || responseModel
}

export function applyNewApiModelSelection(db: {
    aiModel: string
    subModel: string
    forceReplaceUrl: string
    proxyKey: string
    customProxyRequestModel: string
    customAPIFormat: LLMFormat
    autofillRequestUrl: boolean
    seperateModelsForAxModels: boolean
    customTokenizer?: string
    reverseProxyOobaMode?: boolean
    enableCustomFlags?: boolean
    sdProvider?: string
    hypaModel?: string
    supaModelType?: string
    translatorType?: string
    requestLocation?: string
    localNetworkMode?: boolean
    usePlainFetch?: boolean
    enableRemoteSaving?: boolean
    forceProxyAsOpenAI?: boolean
}, group = getNewApiGroup(), model = getNewApiModel()) {
    if(!bridgeEnabled){
        return
    }
    localStorage.setItem('new-api.group', group)
    localStorage.setItem('new-api.model', model)
    db.aiModel = 'reverse_proxy'
    db.subModel = 'reverse_proxy'
    db.forceReplaceUrl = '/api/new-api/chat/completions'
    db.proxyKey = ''
    db.customProxyRequestModel = model || 'new-api'
    db.customAPIFormat = 0 as LLMFormat
    db.autofillRequestUrl = false
    db.seperateModelsForAxModels = false
    db.customTokenizer = 'tiktoken'
    db.reverseProxyOobaMode = false
    db.enableCustomFlags = false
    db.sdProvider = ''
    if(['custom', 'ada', 'openai3small', 'openai3large', 'voyageContext3'].includes(db.hypaModel)){
        db.hypaModel = 'MiniLM'
    }
    if(db.supaModelType && !['none', 'subModel', 'distilbart'].includes(db.supaModelType)){
        db.supaModelType = 'subModel'
    }
    if(['deepl', 'deeplX'].includes(db.translatorType)){
        db.translatorType = 'bergamot'
    }
    db.requestLocation = ''
    db.localNetworkMode = false
    db.usePlainFetch = false
    db.enableRemoteSaving = false
    db.forceProxyAsOpenAI = false
}

export async function getNewApiChatHeaders(
    headers: Record<string, string> = {},
    selection: NewApiChatSelection = getNewApiSelection()
): Promise<Record<string, string>> {
    return await getNewApiHeaders({
        ...headers,
        'X-New-API-Group': selection.group,
        'X-New-API-Model': selection.model
    })
}

export function isNewApiChatUrl(url: string): boolean {
    if(!bridgeEnabled || typeof window === 'undefined'){
        return false
    }
    const target = new URL(url, window.location.origin)
    return target.origin === window.location.origin && target.pathname === '/api/new-api/chat/completions'
}

export async function fetchNewApiAccount(group = getNewApiGroup()): Promise<NewApiAccountResponse> {
    const query = group ? `?group=${encodeURIComponent(group)}` : ''
    const response = await fetch(`/api/new-api/account${query}`, {
        credentials: 'same-origin',
        headers: await getNewApiHeaders()
    })
    if(!response.ok){
        throw new Error(`New API account request failed (${response.status}).`)
    }
    const account = await response.json() as NewApiAccountResponse
    systemName = account.systemName || ''
    if(systemName){
        document.title = `${systemName} | RisuAI`
    }
    return account
}

export async function refreshNewApiAccountAfterGeneration() {
    const account = await fetchNewApiAccount()
    window.dispatchEvent(new CustomEvent<NewApiAccountResponse>('new-api-account-refresh', {
        detail: account
    }))
    return account
}

export async function initializeNewApiModelSelection(db: Parameters<typeof applyNewApiModelSelection>[0]) {
    const account = await fetchNewApiAccount()
    const group = account.group
    const model = account.models.includes(getNewApiModel())
        ? getNewApiModel()
        : account.models[0] || ''
    applyNewApiModelSelection(db, group, model)
    return account
}

export function getNewApiIdentity(): NewApiIdentity | null {
    return identity
}

export function getNewApiPortalUrl(path: '/wallet' | '/usage-logs' | '/dashboard' | '/sign-in'): string {
    const base = identity?.portalUrl || import.meta.env.VITE_NEW_API_PORTAL_URL
    const target = path === '/sign-in'
        ? `${path}?${new URLSearchParams({ redirect: '/tavern' })}`
        : path
    try {
        return new URL(target, base).toString()
    } catch {
        return target
    }
}

export async function logoutNewApi(): Promise<void> {
    if(!bridgeEnabled){
        return
    }
    const response = await fetch('/api/user/auth/new-api/logout', {
        method: 'POST',
        credentials: 'same-origin',
        headers: await getNewApiHeaders()
    })
    if(!response.ok){
        throw new Error(`New API logout failed (${response.status}).`)
    }
    identity = null
    if(refreshTimer){
        clearTimeout(refreshTimer)
        refreshTimer = null
    }
}
