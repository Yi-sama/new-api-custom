import { getStudioGroups, managedStudio, type StudioGroup, type StudioParameterProfile } from "@/lib/new-api-studio";
import { imageSizePresets } from "@/lib/media-size";

export type { StudioModelParameters, StudioParameterProfile } from "@/lib/new-api-studio";

type StudioModelSelection = {
    model?: string;
    imageModel?: string;
    videoModel?: string;
};

export const webImageParameterProfile: StudioParameterProfile = {
    aspect_ratios: ["1:1", "9:16", "16:9", "3:4", "4:3", "3:2", "2:3", "5:4", "4:5", "21:9", "4:1", "1:4", "8:1", "1:8"],
    sizes: ["0.5K", "1K", "2K", "4K"],
    counts: [1, 2, 3, 4],
};

export const defaultImageParameterProfile: StudioParameterProfile = {
    aspect_ratios: ["1:1", "9:16", "16:9", "3:4", "4:3", "3:2", "2:3", "21:9", "9:21"],
    sizes: ["1K", "2K", "4K"],
    qualities: ["low", "medium", "high"],
    counts: [1],
};

export function isImageResolutionProfile(profile: StudioParameterProfile | undefined): boolean {
    return profile === webImageParameterProfile || profile === defaultImageParameterProfile;
}

export function getManagedImageParameterProfile(config: StudioModelSelection): StudioParameterProfile | undefined {
    const profile = getManagedStudioParameterProfile(config, "image");
    if (!profile || Object.values(profile).some((options) => options?.length)) return profile;
    const selected = getManagedStudioParameterProfile({ model: config.model }, "image") ? config.model : config.imageModel;
    const model = selected?.slice(selected.indexOf("::") + 2);
    if (model === "gpt-image-2.5-web") return webImageParameterProfile;
    return defaultImageParameterProfile;
}

export function managedImageParameters(config: StudioModelSelection & { count: string; size: string; quality: string; studioAspectRatio?: string }): Record<string, string | number> {
    const profile = getManagedImageParameterProfile(config);
    const parameters: Record<string, string | number> = {};
    const count = Number(config.count);
    if (config.count.trim() && profile?.counts?.includes(count)) parameters.n = count;
    const size = profile?.sizes?.includes(config.size) && config.size !== "auto" ? config.size : "";
    const ratio = config.studioAspectRatio && profile?.aspect_ratios?.includes(config.studioAspectRatio) && config.studioAspectRatio !== "auto" ? config.studioAspectRatio : "";
    if (profile === webImageParameterProfile) {
        // The web adapter consumes size as a prompt hint; it ignores aspect_ratio.
        if (size && ratio) {
            const edge = Number(size.slice(0, -1)) * 1024;
            const [width, height] = ratio.split(":").map(Number);
            parameters.size = `${Math.round(edge * Math.min(1, width / height))}x${Math.round(edge * Math.min(1, height / width))}`;
        } else if (size || ratio) {
            parameters.size = size || ratio;
        }
    } else if (profile === defaultImageParameterProfile) {
        // Serialize the shared resolution presets as pixel dimensions.
        if (size || ratio) parameters.size = imageSizePresets[(size || "1K").toLowerCase()][ratio || "1:1"];
        if (profile.qualities?.includes(config.quality)) parameters.quality = config.quality;
    } else {
        if (size) parameters.size = size;
        if (ratio) parameters.aspect_ratio = ratio;
        if (config.quality !== "auto" && profile?.qualities?.includes(config.quality)) parameters.quality = config.quality;
    }
    return parameters;
}

export function getManagedStudioParameterProfile(config: StudioModelSelection, capability: "image" | "video"): StudioParameterProfile | undefined {
    if (!managedStudio) return undefined;
    const catalog = getStudioGroups();
    const selected = resolveStudioParameterProfile(catalog, config.model || "", capability);
    if (selected) return selected;
    return resolveStudioParameterProfile(catalog, (capability === "image" ? config.imageModel : config.videoModel) || "", capability);
}

export function resolveStudioParameterProfile(catalog: StudioGroup[], selectedModel: string, capability: "image" | "video"): StudioParameterProfile | undefined {
    const separator = selectedModel.indexOf("::");
    if (!selectedModel.startsWith("site:") || separator < 0) return undefined;
    let group: string;
    try {
        group = decodeURIComponent(selectedModel.slice("site:".length, separator));
    } catch {
        return undefined;
    }
    const modelId = selectedModel.slice(separator + 2);
    const selected = catalog.find((entry) => entry.id === group)?.models.find((entry) => entry.id === modelId);
    if (!selected?.capabilities.includes(capability)) return undefined;
    const parameters = selected.parameters;
    // An available model without metadata must not inherit another model's profile.
    if (!parameters) return {};
    return mergeStudioParameterProfile(parameters[capability], parameters.groups?.[group]?.[capability]);
}

export function profileOptions<T>(profile: StudioParameterProfile | undefined, key: keyof StudioParameterProfile): T[] | undefined {
    const values = profile?.[key];
    return Array.isArray(values) && values.length > 0 ? values as T[] : undefined;
}

function mergeStudioParameterProfile(base: StudioParameterProfile | undefined, override: StudioParameterProfile | undefined): StudioParameterProfile | undefined {
    if (!base && !override) return {};
    return {
        ...base,
        ...Object.fromEntries(Object.entries(override || {}).filter(([, value]) => Array.isArray(value) && value.length > 0)),
    };
}
