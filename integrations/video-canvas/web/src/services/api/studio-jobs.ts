import i18n from "@/i18n";
import { getValidStudioSession, notifyStudio, studioFetch, studioGroupHeaders } from "@/lib/new-api-studio";

export type StudioJobKind = "image" | "video";
export type StudioJobStatus = "queued" | "running" | "submitted" | "succeeded" | "failed" | "uncertain";
export type StudioJob = {
    id: string;
    request_id: string;
    kind: StudioJobKind;
    operation: string;
    group: string;
    model: string;
    prompt: string;
    parameters: Record<string, unknown>;
    status: StudioJobStatus;
    created_at: number;
    updated_at: number;
    error?: string;
    task_id?: string;
};
export type StudioJobReference = { id?: string; requestId: string };
export type StudioJobOptions = {
    signal?: AbortSignal;
    idempotencyKey?: string;
    onRequestCreated?: (requestId: string) => void | Promise<void>;
    onJobCreated?: (job: StudioJob) => void | Promise<void>;
};
type StudioJobListOptions = Pick<StudioJobOptions, "signal"> & { page?: number; requestId?: string };
type StudioEnvelope<T> = { success: boolean; data?: T; message?: string; error?: { message?: string } };

export const studioJobText = (key: string, detail?: string) => {
    const defaults: Record<string, string> = {
        queryInterrupted: "任务查询中断，未重新提交。请继续查询原任务。",
        submissionUncertain: "任务提交结果未确认，未重新提交。请继续查询原请求。",
        uncertain: "任务状态暂不确定，未重新提交。请继续查询原任务。",
        generationFailed: "任务生成失败。",
        noImage: "任务尚无可读取的图片结果。",
        requestFailed: "任务请求失败。",
    };
    const text = i18n.t(`studioJobs.${key}`, { defaultValue: defaults[key] || key });
    return detail ? `${text} ${detail}` : text;
};

export class StudioJobInterruptedError extends Error {
    constructor(readonly jobId: string, detail?: string) {
        super(studioJobText("queryInterrupted", detail));
        this.name = "StudioJobInterruptedError";
    }
}

export class StudioJobSubmissionUncertainError extends Error {
    constructor(readonly requestId: string, detail?: string) {
        super(studioJobText("submissionUncertain", detail));
        this.name = "StudioJobSubmissionUncertainError";
    }
}

export class StudioJobFailedError extends Error {
    constructor(readonly jobId: string, detail?: string) {
        super(detail || studioJobText("generationFailed"));
        this.name = "StudioJobFailedError";
    }
}

class StudioRequestRejectedError extends Error {}

export const isStudioJobInterrupted = (error: unknown): error is StudioJobInterruptedError => error instanceof StudioJobInterruptedError;
export const isStudioJobSubmissionUncertain = (error: unknown): error is StudioJobSubmissionUncertainError => error instanceof StudioJobSubmissionUncertainError;
export const isStudioJobFailed = (error: unknown): error is StudioJobFailedError => error instanceof StudioJobFailedError;

export async function submitStudioJob(
    path: "/jobs/images/generations" | "/jobs/images/edits" | "/jobs/videos",
    body: BodyInit,
    options: StudioJobOptions & { group?: string; contentType?: string } = {},
) {
    await getValidStudioSession();
    const requestId = options.idempotencyKey || crypto.randomUUID();
    const headers = new Headers(studioGroupHeaders(options.group));
    headers.set("Idempotency-Key", requestId);
    if (options.contentType) headers.set("Content-Type", options.contentType);
    options.signal?.throwIfAborted();
    await options.onRequestCreated?.(requestId);
    let job: StudioJob;
    try {
        const response = await studioFetch(path, { method: "POST", body, headers, signal: options.signal });
        job = await readEnvelope<StudioJob>(response);
    } catch (error) {
        // Once dispatched, a lost/aborted response cannot prove that the server rejected the job.
        if (error instanceof StudioRequestRejectedError) throw error;
        throw new StudioJobSubmissionUncertainError(requestId);
    }
    try {
        await options.onJobCreated?.(job);
    } catch {
        throw new StudioJobInterruptedError(job.id);
    }
    return job;
}

export async function getStudioJob(id: string, options: Pick<StudioJobOptions, "signal"> = {}) {
    const job = await readEnvelope<StudioJob>(await studioFetch(`/jobs/${encodeURIComponent(id)}`, { signal: options.signal }));
    if (job.status === "succeeded" || job.status === "failed") notifyStudio("new-api-video-account-changed");
    return job;
}

export async function listStudioJobs(kind: StudioJobKind, options: StudioJobListOptions = {}) {
    const query = new URLSearchParams({ kind, p: String(options.page || 1), page_size: "20" });
    if (options.requestId) query.set("request_id", options.requestId);
    return readEnvelope<{ items: StudioJob[]; total: number }>(await studioFetch(`/jobs?${query}`, { signal: options.signal }));
}

export async function resolveStudioJobReference(kind: StudioJobKind, reference: StudioJobReference, options: Pick<StudioJobOptions, "signal"> = {}) {
    if (reference.id) return reference.id;
    try {
        const job = (await listStudioJobs(kind, { ...options, requestId: reference.requestId })).items.find((item) => item.request_id === reference.requestId);
        if (job) return job.id;
    } catch {
        // A failed lookup or an empty result is not permission to repeat a paid POST.
    }
    throw new StudioJobSubmissionUncertainError(reference.requestId);
}

export async function waitForStudioJob(id: string, options: Pick<StudioJobOptions, "signal"> = {}) {
    for (;;) {
        let job: StudioJob;
        try {
            job = await getStudioJob(id, options);
        } catch {
            throw new StudioJobInterruptedError(id);
        }
        if (job.status === "succeeded") return job;
        if (job.status === "failed") throw new StudioJobFailedError(id, job.error);
        if (job.status === "uncertain") throw new StudioJobInterruptedError(id, job.error);
        try {
            await studioJobPollDelay(options.signal);
        } catch {
            throw new StudioJobInterruptedError(id);
        }
    }
}

export async function fetchStudioImageJobResult(id: string, options: Pick<StudioJobOptions, "signal"> = {}) {
    await waitForStudioJob(id, options);
    try {
        const response = await studioFetch(`/jobs/${encodeURIComponent(id)}/result`, { signal: options.signal });
        if (!response.ok) throw new Error(studioJobText("requestFailed"));
        const payload = await response.json() as { data?: Array<{ url: string; revised_prompt?: string }> };
        if (!payload.data?.length) throw new Error(studioJobText("noImage"));
        return await Promise.all(payload.data.map(async (item, index) => {
            const path = `/jobs/${encodeURIComponent(id)}/assets/${index}`;
            const asset = await studioFetch(path, { signal: options.signal });
            if (!asset.ok) throw new Error(studioJobText("requestFailed"));
            return { id: `${id}:${index}`, dataUrl: await blobToDataUrl(await asset.blob()), revised_prompt: item.revised_prompt };
        }));
    } catch {
        throw new StudioJobInterruptedError(id);
    }
}

export async function fetchStudioVideoJobResult(id: string, options: Pick<StudioJobOptions, "signal"> = {}) {
    try {
        const response = await studioFetch(`/jobs/${encodeURIComponent(id)}/result`, { signal: options.signal });
        if (!response.ok) throw new Error(studioJobText("requestFailed"));
        const blob = await response.blob();
        if (!blob.size || blob.type.includes("json")) throw new Error(studioJobText("requestFailed"));
        notifyStudio("new-api-video-account-changed");
        return blob;
    } catch {
        throw new StudioJobInterruptedError(id);
    }
}

async function readEnvelope<T>(response: Response): Promise<T> {
    const payload = await response.json() as StudioEnvelope<T>;
    if (!response.ok || payload.success !== true || !payload.data) {
        const message = payload.message || payload.error?.message || studioJobText("requestFailed");
        if (response.status >= 400 && response.status < 500) throw new StudioRequestRejectedError(message);
        throw new Error(message);
    }
    return payload.data;
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onerror = () => reject(reader.error);
        reader.onload = () => resolve(String(reader.result));
        reader.readAsDataURL(blob);
    });
}

export function studioJobPollDelay(signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        signal?.throwIfAborted();
        const abort = () => {
            clearTimeout(timer);
            signal?.removeEventListener("abort", abort);
            reject(new DOMException("Aborted", "AbortError"));
        };
        const timer = setTimeout(() => {
            signal?.removeEventListener("abort", abort);
            resolve();
        }, 2500);
        signal?.addEventListener("abort", abort, { once: true });
    });
}
