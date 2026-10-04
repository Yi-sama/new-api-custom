import { getNewApiPortalUrl, initializeNewApiBridge } from "./ts/newApiBridge";

function showNewApiSignIn(error: unknown) {
    const preloading = document.getElementById('preloading')
    if(!preloading){
        return
    }
    preloading.replaceChildren()
    const message = document.createElement('span')
    message.className = 'text-sm mt-2 text-textcolor2'
    message.textContent = error instanceof Error ? error.message : 'New API sign-in is required.'
    const signIn = document.createElement('a')
    signIn.className = 'bg-selected p-2 rounded-md mt-3 hover:bg-blue-500 transition-colors text-textcolor'
    signIn.href = getNewApiPortalUrl('/sign-in')
    signIn.textContent = 'Sign in'
    preloading.append(message, signIn)
}

async function startApplication() {
    await initializeNewApiBridge()

    await import("./ts/polyfill")
    await import("core-js/actual")
    await import("./ts/storage/database.svelte")
    const [
        { default: App },
        { loadData },
        { initHotkey },
        { preLoadCheck },
        { mount }
    ] = await Promise.all([
        import("./App.svelte"),
        import("./ts/bootstrap"),
        import("./ts/hotkey"),
        import("./preload"),
        import("svelte")
    ])

    window.addEventListener('vite:preloadError', (event) => {
        console.error("Chunk load error detected:", event)
        alert("The server has been updated or the network connection has been lost. Please refresh the page.")
    })

    preLoadCheck()
    const app = mount(App, {
        target: document.getElementById("app"),
    })
    void loadData()
    initHotkey()
    document.getElementById('preloading')?.remove()
    return app
}

let app: Awaited<ReturnType<typeof startApplication>> | null = null
try {
    app = await startApplication()
} catch(error) {
    console.error('New API startup failed:', error)
    showNewApiSignIn(error)
}

export default app
