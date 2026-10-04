import { initializeStudio, notifyStudio } from "@/lib/new-api-studio";

// Establish the account before importing any persisted canvas or media stores.
initializeStudio()
    .then(() => import("./app"))
    .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : "Unable to open Video Studio.";
        notifyStudio("new-api-video-error", message);
        const root = document.getElementById("root");
        if (root) {
            root.setAttribute("role", "alert");
            root.style.padding = "24px";
            root.textContent = message;
        }
    });
