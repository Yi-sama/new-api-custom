import localforage from "localforage";
import type { StateStorage } from "zustand/middleware";
import { studioStorageName } from "@/lib/new-api-studio";

localforage.config({
    name: studioStorageName("infinite-canvas"),
    storeName: "app_state",
});

export const localForageStorage: StateStorage = {
    getItem: async (name) => {
        if (typeof window === "undefined") return null;
        try {
            return (await localforage.getItem<string>(name)) || null;
        } catch {
            return window.localStorage.getItem(studioStorageName(name));
        }
    },
    setItem: async (name, value) => {
        if (typeof window === "undefined") return;
        try {
            await localforage.setItem(name, value);
        } catch {
            window.localStorage.setItem(studioStorageName(name), value);
        }
    },
    removeItem: async (name) => {
        if (typeof window === "undefined") return;
        try {
            await localforage.removeItem(name);
        } catch {
            window.localStorage.removeItem(studioStorageName(name));
        }
    },
};
