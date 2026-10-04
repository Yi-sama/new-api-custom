import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "antd";
import { ChevronDown, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { managedStudio } from "@/lib/new-api-studio";
import { listStudioJobs, type StudioJob, type StudioJobKind } from "@/services/api/studio-jobs";

export function useStudioJobHistory(kind: StudioJobKind) {
    const [jobs, setJobs] = useState<StudioJob[]>([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(false);
    const pageRef = useRef(0);
    const busyRef = useRef(false);
    const load = useCallback(async (more: boolean) => {
        if (!managedStudio || busyRef.current) return;
        busyRef.current = true;
        setLoading(true);
        try {
            const page = more ? pageRef.current + 1 : 1;
            const result = await listStudioJobs(kind, { page });
            setJobs((previous) => [...new Map([...previous, ...result.items].map((job) => [job.id, job])).values()].sort((a, b) => b.created_at - a.created_at));
            pageRef.current = more ? page : Math.max(1, pageRef.current);
            setTotal(result.total);
            setError(false);
        } catch {
            setError(true);
        } finally {
            busyRef.current = false;
            setLoading(false);
        }
    }, [kind]);
    const refresh = useCallback(() => load(false), [load]);
    const loadMore = useCallback(() => load(true), [load]);
    useEffect(() => { void refresh(); }, [refresh]);
    return { jobs, total, loading, error, refresh, loadMore };
}

export function StudioHistoryControls({ history }: { history: ReturnType<typeof useStudioJobHistory> }) {
    const { t } = useTranslation();
    if (!managedStudio) return null;
    return (
        <div className="my-3 space-y-2">
            {history.error ? <p role="status" className="text-xs">{t("studioJobs.historyUnavailable", { defaultValue: "任务记录暂时无法读取，请刷新查询。" })}</p> : null}
            <div className="flex gap-2">
                <Button size="small" icon={<RefreshCw className="size-3.5" />} loading={history.loading} onClick={() => void history.refresh()}>{t("studioJobs.refresh", { defaultValue: "刷新任务" })}</Button>
                {history.jobs.length < history.total ? <Button size="small" icon={<ChevronDown className="size-3.5" />} disabled={history.loading} onClick={() => void history.loadMore()}>{t("studioJobs.loadMore", { defaultValue: "加载更多" })}</Button> : null}
            </div>
        </div>
    );
}

export function StudioTaskPaused({ error, onContinue, busy }: { error?: string; onContinue: () => void; busy?: boolean }) {
    const { t } = useTranslation();
    return (
        <div className="flex min-h-40 flex-col items-center justify-center gap-3 rounded-lg border border-current/20 p-4 text-center">
            <span className="text-sm">{t("studioJobs.paused", { defaultValue: "待继续查询" })}</span>
            <p className="max-w-full break-words text-xs">{error || t("studioJobs.queryInterrupted", { defaultValue: "任务查询中断，未重新提交。请继续查询原任务。" })}</p>
            <Button size="small" icon={<RefreshCw className="size-3.5" />} loading={busy} onClick={onContinue}>{t("studioJobs.continueQuery", { defaultValue: "继续查询" })}</Button>
        </div>
    );
}
