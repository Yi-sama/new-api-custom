export type StudioSession = {
    user_id: number;
    username: string;
    token: string;
    expires_at: number;
};

export type StudioAccount = {
    user_id: number;
    username: string;
    display_name: string;
    group: string;
    balance: string | null;
    used: string | null;
    loading: boolean;
};

export type StudioCapability = "image" | "video" | "text" | "audio";
export type StudioParameterProfile = {
    sizes?: string[];
    aspect_ratios?: string[];
    qualities?: string[];
    counts?: number[];
    durations?: number[];
    resolutions?: string[];
    modes?: string[];
    generate_audio?: boolean[];
    watermarks?: boolean[];
    fps?: number[];
};
export type StudioModelParameters = {
    image?: StudioParameterProfile;
    video?: StudioParameterProfile;
    groups?: Record<
        string,
        {
            image?: StudioParameterProfile;
            video?: StudioParameterProfile;
        }
    >;
};
export type StudioGroup = {
    id: string;
    description: string;
    ratio: number;
    models: Array<{ id: string; capabilities: StudioCapability[]; parameters?: StudioModelParameters }>;
};

export const managedStudio = import.meta.env?.VITE_NEW_API_STUDIO === "1";
export const studioApiPath = "/api/video-studio";
let session: StudioSession | null = null;
let groups: StudioGroup[] = [];
let renewal: Promise<StudioSession> | null = null;

function isTrustedStudioMessage(event: { origin: string; source: unknown; data: unknown }, origin: string, parent: unknown) {
    return event.origin === origin && event.source === parent && Boolean(event.data) && typeof event.data === "object";
}

export function readStudioSessionMessage(event: { origin: string; source: unknown; data: unknown }, origin: string, parent: unknown): StudioSession | null {
    if (!isTrustedStudioMessage(event, origin, parent)) return null;
    const message = event.data as { type?: unknown; session?: Partial<StudioSession> };
    const value = message.session;
    if (message.type !== "new-api-video-session" || !value || typeof value !== "object") return null;
    if (!Number.isSafeInteger(value.user_id) || value.user_id! <= 0 || typeof value.username !== "string") return null;
    if (typeof value.token !== "string" || !value.token || typeof value.expires_at !== "number" || value.expires_at <= Date.now() / 1000) return null;
    return { user_id: value.user_id!, username: value.username, token: value.token, expires_at: value.expires_at };
}

export function studioStorageName(name: string, userId = session?.user_id) {
    return userId ? `new-api-${userId}:${name}` : name;
}

export function getStudioSession() {
    if (!session) throw new Error("Video Studio session is unavailable.");
    return session;
}

export async function getValidStudioSession(): Promise<StudioSession> {
    const current = getStudioSession();
    if (current.expires_at > Date.now() / 1000 || !managedStudio) return current;
    if (renewal) return renewal;
    if (window.parent === window) throw new Error("Open Video Studio from your New API account.");

    renewal = new Promise<StudioSession>((resolve, reject) => {
        const finish = (value: StudioSession | Error) => {
            window.removeEventListener("message", receive);
            if (value instanceof Error) reject(value);
            else resolve(value);
        };
        const receive = (event: MessageEvent) => {
            if (!isTrustedStudioMessage(event, window.location.origin, window.parent)) return;
            const message = event.data as { type?: unknown; message?: unknown };
            if (message.type === "new-api-video-session-error" && typeof message.message === "string" && message.message) {
                finish(new Error(message.message));
                return;
            }
            if (message.type !== "new-api-video-session") return;
            const next = readStudioSessionMessage(event, window.location.origin, window.parent);
            if (!next) {
                finish(new Error("Video Studio returned an invalid renewed session. Please sign in again."));
                return;
            }
            if (next.user_id !== current.user_id || next.username !== current.username) {
                finish(new Error("Video Studio session changed accounts. Please sign in again."));
                return;
            }
            session = next;
            finish(next);
        };
        window.addEventListener("message", receive);
        window.parent.postMessage({ type: "new-api-video-renew-session" }, window.location.origin);
    });
    try {
        return await renewal;
    } finally {
        renewal = null;
    }
}

export async function studioFetch(path: string, init?: RequestInit): Promise<Response> {
    if (!managedStudio) return fetch(path, init);
    const current = await getValidStudioSession();
    const request = (token: string) => {
        const headers = new Headers(init?.headers);
        headers.set("Authorization", `Bearer ${token}`);
        return fetch(`${studioApiPath}${path}`, { ...init, headers });
    };
    const response = await request(current.token);
    if (response.status !== 401) return response;
    if (session?.token === current.token) session = { ...session, expires_at: 0 };
    const renewed = await getValidStudioSession();
    if ((init?.method ?? "GET").toUpperCase() !== "GET") return response;
    const retried = await request(renewed.token);
    if (retried.status === 401 && session?.token === renewed.token) session = { ...session, expires_at: 0 };
    return retried;
}

export function getStudioGroups() {
    return groups;
}

export function studioChannelId(group: string) {
    return `site:${encodeURIComponent(group)}`;
}

export function filterStudioGroups(catalog: StudioGroup[], capability?: StudioCapability): StudioGroup[] {
    return catalog.map((group) => ({ ...group, models: group.models.filter((model) => !capability || model.capabilities.includes(capability)) })).filter((group) => group.models.length > 0);
}

export function studioGroupHeaders(group?: string): Record<string, string> {
    return group ? { "X-New-Api-Group": encodeURIComponent(group) } : {};
}

export function readStudioAccountMessage(event: { origin: string; source: unknown; data: unknown }, origin: string, parent: unknown, userId: number): StudioAccount | null {
    if (event.origin !== origin || event.source !== parent || !event.data || typeof event.data !== "object") return null;
    const message = event.data as { type?: unknown; account?: Partial<StudioAccount> };
    const value = message.account;
    if (message.type !== "new-api-video-account" || !value || value.user_id !== userId) return null;
    if (typeof value.username !== "string" || typeof value.display_name !== "string" || typeof value.group !== "string") return null;
    if ((value.balance !== null && typeof value.balance !== "string") || (value.used !== null && typeof value.used !== "string") || typeof value.loading !== "boolean") return null;
    return value as StudioAccount;
}

export function notifyStudio(type: "new-api-video-ready" | "new-api-video-error" | "new-api-video-account-changed", message?: string) {
    if (managedStudio) window.parent.postMessage({ type, message }, window.location.origin);
}

// Only New API supplies authoritative balances, including its currency formatting.
export function withStudioAccountRefresh<Args extends unknown[], Result>(request: (...args: Args) => Promise<Result>) {
    return async (...args: Args): Promise<Result> => {
        try {
            if (managedStudio) await getValidStudioSession();
            return await request(...args);
        } finally {
            notifyStudio("new-api-video-account-changed");
        }
    };
}

export function requestStudioAccountAction(action: "dashboard" | "sign-out") {
    if (managedStudio) window.parent.postMessage({ type: "new-api-video-account-action", action }, window.location.origin);
}

export async function initializeStudio() {
    if (!managedStudio) return;
    if (window.parent === window) throw new Error("Open Video Studio from your New API account.");
    session = await new Promise<StudioSession>((resolve) => {
        const receive = (event: MessageEvent) => {
            const value = readStudioSessionMessage(event, window.location.origin, window.parent);
            if (!value) return;
            window.removeEventListener("message", receive);
            resolve(value);
        };
        window.addEventListener("message", receive);
        window.parent.postMessage({ type: "new-api-video-request-session" }, window.location.origin);
    });
    const response = await studioFetch("/catalog", { cache: "no-store" });
    if (!response.ok) throw new Error(`Video Studio model access failed (${response.status}).`);
    const body: { success?: boolean; data?: { groups?: StudioGroup[] } } = await response.json();
    if (!body.success || !Array.isArray(body.data?.groups)) throw new Error("Video Studio returned an invalid model catalog.");
    groups = body.data.groups;
}
