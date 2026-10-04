import { useTranslation } from "react-i18next";
import { type CanvasTheme } from "@/lib/canvas-theme";
import { getManagedStudioParameterProfile } from "@/lib/studio-model-parameters";
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

export function ManagedImageSettings(props: ManagedSettingsProps<ManagedImageSettingKey>) {
    const { t } = useTranslation();
    const profile = getManagedStudioParameterProfile(props.config, "image");
    return (
        <div className={props.className} style={{ color: props.theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
            {props.showTitle ? <div className="text-lg font-semibold">{t("settingsPanels.image.title")}</div> : null}
            <ParameterChoice label={t("settingsPanels.image.quality")} options={profile?.qualities} value={props.config.quality} theme={props.theme} onChange={(value) => props.onConfigChange("quality", value)} />
            <ParameterChoice label={t("settingsPanels.image.size")} options={profile?.sizes} value={props.config.size} theme={props.theme} onChange={(value) => props.onConfigChange("size", value)} />
            <ParameterChoice label={t("settingsPanels.image.aspectRatio")} options={profile?.aspect_ratios} value={props.config.studioAspectRatio} theme={props.theme} onChange={(value) => props.onConfigChange("studioAspectRatio", value)} />
            <ParameterChoice label={t("settingsPanels.image.count")} options={profile?.counts} value={props.config.count} theme={props.theme} onChange={(value) => props.onConfigChange("count", value)} />
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
                    if (option === "true") label = t("studioJobs.enabled");
                    if (option === "false") label = t("studioJobs.disabled");
                    return <option key={option} value={option}>{label}</option>;
                })}
            </select>
        </label>
    );
}
