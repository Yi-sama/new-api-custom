import { FileText, ImagePlus, Images, Maximize2, Settings2, Video } from "lucide-react";
import { managedStudio } from "@/lib/new-api-studio";

const tools = [
    {
        slug: "canvas",
        icon: Maximize2,
    },
    {
        slug: "image",
        icon: ImagePlus,
    },
    {
        slug: "video",
        icon: Video,
    },
    {
        slug: "prompts",
        icon: FileText,
    },
    {
        slug: "assets",
        icon: Images,
    },
    {
        slug: "config",
        icon: Settings2,
    },
] as const;

export const navigationTools = managedStudio ? tools.filter((tool) => tool.slug !== "config") : tools;
export type NavigationToolSlug = (typeof tools)[number]["slug"];
