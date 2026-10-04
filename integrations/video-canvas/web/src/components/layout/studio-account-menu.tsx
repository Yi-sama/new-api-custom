import { useEffect, useState } from "react";
import { Dropdown } from "antd";
import { ArrowLeft, BookOpen, ListVideo, LogOut, Receipt, RefreshCw, Tags, UserRound, Wallet } from "lucide-react";
import { useTranslation } from "react-i18next";

import { getStudioSession, notifyStudio, readStudioAccountMessage, requestStudioAccountAction, type StudioAccount } from "@/lib/new-api-studio";
import { DOCS_URL } from "@/constant/env";

export function StudioAccountMenu() {
    const { t } = useTranslation();
    const [account, setAccount] = useState<StudioAccount | null>(null);
    const [open, setOpen] = useState(false);
    const session = getStudioSession();
    useEffect(() => {
        const receive = (event: MessageEvent) => {
            const value = readStudioAccountMessage(event, window.location.origin, window.parent, session.user_id);
            if (value) setAccount(value);
        };
        window.addEventListener("message", receive);
        notifyStudio("new-api-video-account-changed");
        return () => window.removeEventListener("message", receive);
    }, [session.user_id]);

    const loading = !account || account.loading;
    const balance = account?.balance ?? t(loading ? "studioAccount.loading" : "studioAccount.unavailable");
    const iconClass = "size-4";
    const links = [
        { key: "recharge", href: "/wallet", icon: Wallet },
        { key: "profile", href: "/profile", icon: UserRound },
        { key: "usage", href: "/usage-logs/common", icon: Receipt },
        { key: "tasks", href: "/usage-logs/task", icon: ListVideo },
        { key: "pricing", href: "/pricing", icon: Tags },
    ];

    return (
        <div className="inline-flex shrink-0 items-center gap-2">
            <Dropdown
                open={open}
                onOpenChange={setOpen}
                trigger={["click"]}
                placement="bottomRight"
                menu={{ onClick: () => setOpen(false), items: [
                    { key: "identity", disabled: true, label: <div className="max-w-56 whitespace-normal break-words font-medium text-stone-900 dark:text-stone-100">{account?.display_name || session.username}</div> },
                    { key: "balance", disabled: true, label: <div className="flex max-w-56 justify-between gap-4 whitespace-normal break-words text-stone-600 dark:text-stone-300"><span>{t("studioAccount.balance")}</span><span>{balance}</span></div> },
                    { key: "used", disabled: true, label: <div className="flex max-w-56 justify-between gap-4 whitespace-normal break-words text-stone-600 dark:text-stone-300"><span>{t("studioAccount.used")}</span><span>{account?.used ?? t(loading ? "studioAccount.loading" : "studioAccount.unavailable")}</span></div> },
                    { key: "group", disabled: true, label: <div className="max-w-56 whitespace-normal break-words text-stone-600 dark:text-stone-300">{t("studioAccount.group")}: {account?.group || t("studioAccount.unavailable")}</div> },
                    { key: "refresh", icon: <RefreshCw className={iconClass} />, label: t("studioAccount.refresh"), disabled: loading, onClick: () => notifyStudio("new-api-video-account-changed") },
                    { type: "divider" },
                    ...links.map(({ key, href, icon: Icon }) => ({ key, icon: <Icon className={iconClass} />, label: <a href={href} target="_blank" rel="noopener noreferrer">{t(`studioAccount.${key}`)}</a> })),
                    { key: "docs", icon: <BookOpen className={iconClass} />, label: <a href={DOCS_URL} target="_blank" rel="noopener noreferrer">{t("topNav.docs")}</a> },
                    { type: "divider" },
                    { key: "dashboard", icon: <ArrowLeft className={iconClass} />, label: t("studioAccount.dashboard"), onClick: () => requestStudioAccountAction("dashboard") },
                    { key: "sign-out", danger: true, icon: <LogOut className={iconClass} />, label: t("studioAccount.signOut"), onClick: () => requestStudioAccountAction("sign-out") },
                ] }}
            >
                <button type="button" aria-label={t("studioAccount.account")} aria-haspopup="menu" aria-expanded={open} title={account?.display_name || session.username} className="inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-stone-600 transition-colors hover:bg-black/5 hover:text-stone-950 dark:text-stone-300 dark:hover:bg-white/10 dark:hover:text-white">
                    <UserRound className={iconClass} />
                </button>
            </Dropdown>
            <a
                href="/wallet"
                target="_blank"
                rel="noopener noreferrer"
                title={`${t("studioAccount.balance")}: ${balance} (${t("studioAccount.recharge")})`}
                className="inline-flex h-8 w-20 shrink-0 items-center justify-center gap-1.5 rounded-md border border-border bg-secondary! px-2 text-xs font-medium text-secondary-foreground transition-colors hover:bg-accent! focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring sm:w-28 sm:px-3 sm:text-sm"
            >
                <Wallet className="size-4 shrink-0" aria-hidden="true" />
                <output aria-label={t("studioAccount.balance")} aria-busy={loading} className="min-w-0 truncate tabular-nums">{balance}</output>
            </a>
        </div>
    );
}
