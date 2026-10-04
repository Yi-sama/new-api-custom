import { getStudioGroups, managedStudio, type StudioGroup, type StudioParameterProfile } from "@/lib/new-api-studio";

export type { StudioModelParameters, StudioParameterProfile } from "@/lib/new-api-studio";

type StudioModelSelection = {
    model?: string;
    imageModel?: string;
    videoModel?: string;
};

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
