// Midjourney channels reject long prompts (uuyue 900+, osodm 850+), so studio MJ jobs share one cap.
export const MJ_PROMPT_LIMIT = 750;

export function isMjModel(model: string | undefined | null) {
    const name = (model || "").trim().toLowerCase();
    return name.startsWith("midjourney") || name.startsWith("mj-") || name.startsWith("mj_");
}

export function mjPromptLength(prompt: string) {
    return Array.from(prompt).length;
}

export function mjPromptMessage(prompt: string) {
    return `Midjourney 提示词不能超过 ${MJ_PROMPT_LIMIT} 个字符（当前 ${mjPromptLength(prompt)}），请精简后再提交。`;
}

export function assertMjPrompt(model: string | undefined | null, prompt: string) {
    if (isMjModel(model) && mjPromptLength(prompt) > MJ_PROMPT_LIMIT) throw new Error(mjPromptMessage(prompt));
}
