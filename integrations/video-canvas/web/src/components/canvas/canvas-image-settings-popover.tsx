import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Settings2 } from "lucide-react";
import { Button } from "antd";
import { useTranslation } from "react-i18next";

import { ImageSettingsPanel, imageQualityLabel, imageSizeLabel } from "@/components/image-settings-panel";
import { canvasThemes } from "@/lib/canvas-theme";
import { managedStudio } from "@/lib/new-api-studio";
import { defaultImageParameterProfile, getManagedImageParameterProfile, isImageResolutionProfile, managedImageParameters } from "@/lib/studio-model-parameters";
import { useThemeStore } from "@/stores/use-theme-store";
import type { AiConfig } from "@/stores/use-config-store";

type CanvasImageSettingsPopoverProps = {
    config: AiConfig;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
    onMissingConfig?: () => void;
    onOpenChange?: (open: boolean) => void;
    buttonClassName?: string;
    getPopupContainer?: (triggerNode: HTMLElement) => HTMLElement;
    placement?: "topLeft" | "top" | "topRight" | "bottomLeft" | "bottom" | "bottomRight";
    autoAdjustOverflow?: boolean;
};

export function CanvasImageSettingsPopover({ config, onConfigChange, onOpenChange, buttonClassName, placement = "topLeft" }: CanvasImageSettingsPopoverProps) {
    const { t } = useTranslation();
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const buttonRef = useRef<HTMLSpanElement>(null);
    const panelRef = useRef<HTMLDivElement>(null);
    const [open, setOpen] = useState(false);
    const [buttonRect, setButtonRect] = useState<DOMRect | null>(null);
    const quality = config.quality || "auto";
    const count = Math.max(1, Math.min(15, Math.floor(Math.abs(Number(config.count)) || 1)));
    const activeSize = config.size || "auto";
    const profile = getManagedImageParameterProfile(config);
    const parameters = managedImageParameters(config);
    const summary = managedStudio
        ? isImageResolutionProfile(profile)
            ? [parameters.quality ? imageQualityLabel(String(parameters.quality)) : "", profile?.aspect_ratios?.includes(config.studioAspectRatio || "") ? config.studioAspectRatio : profile === defaultImageParameterProfile && parameters.size ? "1:1" : t("settingsPanels.common.auto"), profile?.sizes?.includes(config.size) ? config.size : profile === defaultImageParameterProfile && parameters.size ? "1K" : ""].filter(Boolean).join(" · ")
            : [parameters.quality ? imageQualityLabel(String(parameters.quality)) : "", parameters.aspect_ratio || imageSizeLabel(String(parameters.size || "auto"))].filter(Boolean).join(" · ")
        : `${imageQualityLabel(quality)} · ${imageSizeLabel(activeSize)}`;
    const updateOpen = (nextOpen: boolean) => {
        setOpen(nextOpen);
        onOpenChange?.(nextOpen);
    };

    useEffect(() => {
        if (!open) return;
        const syncPosition = () => setButtonRect(buttonRef.current?.getBoundingClientRect() || null);
        const closeOnOutsidePointer = (event: PointerEvent) => {
            const target = event.target;
            if (!(target instanceof Node)) return;
            if (buttonRef.current?.contains(target) || panelRef.current?.contains(target)) return;
            if (document.activeElement instanceof HTMLElement && panelRef.current?.contains(document.activeElement)) document.activeElement.blur();
            setOpen(false);
            onOpenChange?.(false);
        };

        syncPosition();
        window.addEventListener("resize", syncPosition);
        window.addEventListener("scroll", syncPosition, true);
        window.addEventListener("pointerdown", closeOnOutsidePointer, true);
        return () => {
            window.removeEventListener("resize", syncPosition);
            window.removeEventListener("scroll", syncPosition, true);
            window.removeEventListener("pointerdown", closeOnOutsidePointer, true);
        };
    }, [onOpenChange, open]);

    const panel = open && buttonRect ? <ImageSettingsPortal buttonRect={buttonRect} panelRef={panelRef} placement={placement} theme={theme} config={config} onConfigChange={onConfigChange} /> : null;

    return (
        <>
            <span ref={buttonRef} className="inline-flex min-w-0">
                <Button size="small" type="text" className={buttonClassName || "!h-8 !max-w-[180px] !justify-start !rounded-full !px-2.5"} style={{ background: theme.node.fill, color: theme.node.text }} icon={<Settings2 className="size-3.5" />} onClick={() => updateOpen(!open)}>
                    <span className="truncate">
                        {summary} · {t("canvas.controls.images", { count })}
                    </span>
                </Button>
            </span>
            {panel}
        </>
    );
}

function ImageSettingsPortal({
    buttonRect,
    panelRef,
    placement,
    theme,
    config,
    onConfigChange,
}: {
    buttonRect: DOMRect;
    panelRef: RefObject<HTMLDivElement | null>;
    placement: CanvasImageSettingsPopoverProps["placement"];
    theme: (typeof canvasThemes)[keyof typeof canvasThemes];
    config: AiConfig;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
}) {
    const width = Math.min(356, window.innerWidth - 24);
    const [height, setHeight] = useState(0);
    useLayoutEffect(() => {
        setHeight(panelRef.current?.scrollHeight || 0);
    }, [config, panelRef, width]);
    const gap = 8;
    const margin = 12;
    const alignRight = placement?.endsWith("Right");
    const alignCenter = placement === "top" || placement === "bottom";
    const left = alignCenter ? buttonRect.left + buttonRect.width / 2 - width / 2 : alignRight ? buttonRect.right - width : buttonRect.left;
    const topPlacement = placement?.startsWith("top");
    const panelHeight = Math.min(height, window.innerHeight - margin * 2);
    const verticalSpace = topPlacement ? buttonRect.top - gap - margin : window.innerHeight - buttonRect.bottom - gap - margin;
    const leftSide = buttonRect.left - gap - width;
    const rightSide = buttonRect.right + gap;
    const sideLeft = leftSide >= margin && leftSide + width <= window.innerWidth - margin ? leftSide : rightSide >= margin && rightSide + width <= window.innerWidth - margin ? rightSide : undefined;
    const useSide = verticalSpace < panelHeight && sideLeft !== undefined;
    const style = {
        position: "fixed",
        zIndex: 1200,
        width,
        left: useSide ? sideLeft : Math.max(margin, Math.min(window.innerWidth - width - margin, left)),
        top: Math.max(margin, Math.min(window.innerHeight - panelHeight - margin, useSide ? buttonRect.top : topPlacement ? buttonRect.top - gap - panelHeight : buttonRect.bottom + gap)),
        maxHeight: Math.max(1, window.innerHeight - margin * 2),
        background: theme.toolbar.panel,
        borderRadius: 18,
        boxShadow: "0 18px 54px rgba(28, 25, 23, 0.16)",
        padding: 18,
        overflowY: "auto",
        color: theme.node.text,
    } as const;

    return createPortal(
        <div
            ref={panelRef}
            className="canvas-image-settings-popover"
            style={style}
            onPointerDown={(event) => event.stopPropagation()}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => event.stopPropagation()}
        >
            <ImageSettingsPanel config={config} onConfigChange={(key, value) => onConfigChange(key, value)} theme={theme} className="space-y-4" batchCount />
        </div>,
        document.body,
    );
}
