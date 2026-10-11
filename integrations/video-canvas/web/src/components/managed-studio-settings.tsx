import { useTranslation } from "react-i18next";
import { type CanvasTheme } from "@/lib/canvas-theme";
import { defaultImageParameterProfile, getManagedImageParameterProfile, getManagedStudioParameterProfile, isImageResolutionProfile } from "@/lib/studio-model-parameters";
import type { AiConfig } from "@/stores/use-config-store";

export type ManagedImageSettingKey = "quality" | "size" | "count" | "background" | "studioAspectRatio";
export type ManagedVideoSettingKey = "vquality" | "size" | "videoSeconds" | "videoGenerateAudio" | "videoWatermark" | "videoMode" | "studioAspectRatio" | "videoFps";

type ManagedSettingsProps<Key extends keyof AiConfig> = {
    config: AiConfig;
    theme: CanvasTheme;
    onConfigChange: (key: Key, value: string) => void;
    showTitle?: boolean;
    className?: string;
};

export function ManagedImageSettings(props: ManagedSettingsProps<ManagedImageSettingKey> & { batchCount?: boolean; maxCount?: number }) {
    const { t } = useTranslation();
    const profile = getManagedImageParameterProfile(props.config);
    const pixelSize = profile === defaultImageParameterProfile;
    const ratios = (profile?.aspect_ratios || []).filter((value) => value !== "auto");
    const configuredRatio = ratios.includes(props.config.studioAspectRatio || "") ? props.config.studioAspectRatio : "";
    const configuredSize = profile?.sizes?.includes(props.config.size) ? props.config.size : "";
    const ratio = configuredRatio || (pixelSize && configuredSize ? "1:1" : "");
    const size = configuredSize || (pixelSize && configuredRatio ? "1K" : "");
    const selectRatio = (value: string) => {
        props.onConfigChange("studioAspectRatio", value);
        if (pixelSize) props.onConfigChange("size", value ? size || "1K" : "");
    };
    const selectSize = (value: string) => {
        props.onConfigChange("size", value);
        if (pixelSize) props.onConfigChange("studioAspectRatio", value ? ratio || "1:1" : "");
    };
    const counts = props.batchCount ? Array.from({ length: props.maxCount || 15 }, (_, index) => index + 1) : profile?.counts;
    const count = counts?.includes(Number(props.config.count)) ? props.config.count : String(counts?.[0] || 1);
    return (
        <div className={props.className} style={{ color: props.theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
            {props.showTitle ? <div className="text-lg font-semibold">{t("settingsPanels.image.title")}</div> : null}
            {ratios.length ? (
                <div className="space-y-2">
                    <div className="text-xs font-medium" style={{ color: props.theme.node.muted }}>{t("settingsPanels.image.aspectRatio")}</div>
                    <div className="grid grid-cols-6 gap-1" role="group" aria-label={t("settingsPanels.image.aspectRatio")}>
                        {["", ...ratios].map((value) => {
                            const [width, height] = value.split(":").map(Number);
                            const aspect = width > 0 && height > 0 ? width / height : 1;
                            return (
                                <button key={value} type="button" aria-pressed={ratio === value} className="flex h-16 min-w-0 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-lg text-xs transition hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2"
                                    style={{ background: ratio === value ? props.theme.toolbar.activeBg : "transparent", color: ratio === value ? props.theme.node.text : props.theme.node.muted }}
                                    onClick={() => selectRatio(value)}>
                                    <span className="grid h-7 w-9 place-items-center" aria-hidden="true">
                                        <span style={{ width: 28 * Math.min(1, aspect), height: 28 * Math.min(1, 1 / aspect), border: `1.5px ${value ? "solid" : "dashed"} currentColor`, borderRadius: 2 }} />
                                    </span>
                                    <span>{value || t("settingsPanels.common.auto")}</span>
                                </button>
                            );
                        })}
                    </div>
                </div>
            ) : null}
            {isImageResolutionProfile(profile) ? (
                <div className="space-y-2">
                    <div className="text-xs font-medium" style={{ color: props.theme.node.muted }}>{t("settingsPanels.image.resolution")}</div>
                    <div className="grid gap-1 rounded-lg p-1" style={{ background: props.theme.node.panel, gridTemplateColumns: `repeat(${(profile?.sizes?.length || 0) + 1}, minmax(0, 1fr))` }} role="group" aria-label={t("settingsPanels.image.resolution")}>
                        {["", ...(profile?.sizes || [])].map((value) => {
                            return <button key={value} type="button" aria-pressed={size === value} className="h-8 min-w-0 cursor-pointer rounded-md text-xs transition hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2" style={{ background: size === value ? props.theme.toolbar.activeBg : "transparent", color: props.theme.node.text }} onClick={() => selectSize(value)}>{value || t("settingsPanels.common.auto")}</button>;
                        })}
                    </div>
                </div>
            ) : <ParameterChoice label={t("settingsPanels.image.size")} options={profile?.sizes} value={props.config.size} theme={props.theme} onChange={(value) => props.onConfigChange("size", value)} />}
            <ParameterChoice label={t("settingsPanels.image.quality")} options={profile?.qualities} value={props.config.quality} theme={props.theme} onChange={(value) => props.onConfigChange("quality", value)} />
            {counts?.length ? (
                <label className="flex items-center justify-between gap-3 text-xs font-medium">
                    <span style={{ color: props.theme.node.muted }}>{t("settingsPanels.image.count")}</span>
                    <select aria-label={t("settingsPanels.image.count")} className="h-9 min-w-20 rounded-lg border px-2 text-sm" style={{ color: props.theme.node.text, background: props.theme.node.panel, borderColor: props.theme.node.stroke }} value={count} onChange={(event) => props.onConfigChange("count", event.target.value)}>
                        {counts.map((value) => <option key={value} value={value}>{t("settingsPanels.image.images", { count: value })}</option>)}
                    </select>
                </label>
            ) : null}
            {!profile || Object.values(profile).every((options) => !options?.length) ? <span role="status">{t("settingsPanels.common.auto")}</span> : null}
        </div>
    );
}

export function ManagedVideoSettings(props: ManagedSettingsProps<ManagedVideoSettingKey>) {
    const { t } = useTranslation();
    const profile = getManagedStudioParameterProfile(props.config, "video");
    return (
        <div className={props.className} style={{ color: props.theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
            {props.showTitle ? <div className="text-lg font-semibold">{t("settingsPanels.video.title")}</div> : null}
            <ParameterChoice label={t("settingsPanels.video.quality")} options={profile?.resolutions} value={props.config.vquality} theme={props.theme} onChange={(value) => props.onConfigChange("vquality", value)} />
            <ParameterChoice label={t("settingsPanels.video.size")} options={profile?.sizes} value={props.config.size} theme={props.theme} onChange={(value) => props.onConfigChange("size", value)} />
            <ParameterChoice label={t("settingsPanels.video.ratio")} options={profile?.aspect_ratios} value={props.config.studioAspectRatio} theme={props.theme} onChange={(value) => props.onConfigChange("studioAspectRatio", value)} />
            <ParameterChoice label={t("settingsPanels.video.seconds")} options={profile?.durations} value={props.config.videoSeconds} theme={props.theme} onChange={(value) => props.onConfigChange("videoSeconds", value)} />
            <ParameterChoice label={t("settingsPanels.video.mode")} options={profile?.modes} value={props.config.videoMode} theme={props.theme} onChange={(value) => props.onConfigChange("videoMode", value)} />
            <ParameterChoice label={t("settingsPanels.video.generateAudio", { defaultValue: "Audio" })} options={profile?.generate_audio} value={props.config.videoGenerateAudio} theme={props.theme} onChange={(value) => props.onConfigChange("videoGenerateAudio", value)} />
            <ParameterChoice label={t("settingsPanels.video.watermark", { defaultValue: "Watermark" })} options={profile?.watermarks} value={props.config.videoWatermark} theme={props.theme} onChange={(value) => props.onConfigChange("videoWatermark", value)} />
            <ParameterChoice label="FPS" options={profile?.fps} value={props.config.videoFps} theme={props.theme} onChange={(value) => props.onConfigChange("videoFps", value)} />
            {!profile || Object.values(profile).every((options) => !options?.length) ? <span role="status">{t("settingsPanels.common.auto")}</span> : null}
        </div>
    );
}

function ParameterChoice(props: {
    label: string;
    options?: Array<string | number | boolean>;
    value?: string;
    theme: CanvasTheme;
    onChange: (value: string) => void;
}) {
    const { t } = useTranslation();
    if (!props.options?.length) return null;
    const options = props.options.map(String);
    const value = props.value && options.includes(props.value) ? props.value : "";
    return (
        <label className="flex min-w-0 flex-col gap-2 text-xs font-medium">
            <span style={{ color: props.theme.node.muted }}>{props.label}</span>
            <select className="h-9 w-full min-w-0 rounded-lg border px-2 text-sm" style={{ color: props.theme.node.text, background: props.theme.node.panel, borderColor: props.theme.node.stroke }} value={value} onChange={(event) => props.onChange(event.target.value)}>
                <option value="">{t("settingsPanels.common.auto")}</option>
                {options.map((option) => {
                    let label = option;
                    if (["low", "medium", "high"].includes(option)) label = t(`settingsPanels.common.${option}`);
                    if (option === "true") label = t("studioJobs.enabled");
                    if (option === "false") label = t("studioJobs.disabled");
                    return <option key={option} value={option}>{label}</option>;
                })}
            </select>
        </label>
    );
}
