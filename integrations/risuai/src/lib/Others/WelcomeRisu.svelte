<script lang="ts">
    import { ArrowRight, Send } from "@lucide/svelte";
    import { onMount } from "svelte";
    import { changeLanguage, language } from "src/lang";
    import { setPreset } from "src/ts/storage/database.svelte";
    import { DBState } from 'src/ts/stores.svelte';
    import Chat from "../ChatScreens/Chat.svelte";
    import { prebuiltPresets } from "src/ts/process/templates/templates";
    import { updateTextThemeAndCSS } from "src/ts/gui/colorscheme";
    import { alertError } from "src/ts/alert";
    import Button from "src/lib/UI/GUI/Button.svelte";
    import OptionInput from "src/lib/UI/GUI/OptionInput.svelte";
    import SelectInput from "src/lib/UI/GUI/SelectInput.svelte";
    import {
        applyNewApiModelSelection,
        fetchNewApiAccount,
        formatNewApiBalance,
        formatNewApiTavernCredit,
        getNewApiGroup,
        getNewApiModel,
        getNewApiSiteName,
        isNewApiBridge,
        type NewApiAccountResponse,
    } from "src/ts/newApiBridge";
    import Airisu from '../../etc/Airisu.webp'

    const airisuStyle = `background: url("${Airisu}");background-size: cover;`
    let step = $state(0)
    let provider = $state('')
    let input = $state('')
    let chatLang = $state(0)
    let chatMemorySelection = $state(0)
    const newApiBridgeEnabled = isNewApiBridge()
    let newApiAccount = $state<NewApiAccountResponse | null>(null)
    let newApiGroup = $state(getNewApiGroup())
    let newApiModel = $state(getNewApiModel())
    let newApiLoading = $state(false)
    let newApiError = $state('')
    let newApiRequestId = 0
    let siteName = $derived(getNewApiSiteName(newApiAccount))

    {
        const browserLang = navigator.language
        const browserLangShort = browserLang.split('-')[0]
        const usableLangs = ['de', 'en', 'ko', 'cn', 'vi', 'zh-Hant']
        if(usableLangs.includes(browserLangShort)){
            changeLanguage(browserLangShort)
            DBState.db.language = browserLangShort
            step = 1
        }
    }
    let start = $state(false)

    async function loadNewApiAccount(group = newApiGroup) {
        const requestId = ++newApiRequestId
        newApiLoading = true
        newApiError = ''
        try {
            const account = await fetchNewApiAccount(group)
            if(requestId !== newApiRequestId){
                return
            }
            newApiAccount = account
            newApiGroup = account.group
            if(!account.models.includes(newApiModel)){
                newApiModel = account.models[0] || ''
            }
            if(newApiGroup && newApiModel){
                applyNewApiModelSelection(DBState.db, newApiGroup, newApiModel)
            }
        } catch(error) {
            if(requestId === newApiRequestId){
                newApiError = error instanceof Error ? error.message : String(error)
            }
        } finally {
            if(requestId === newApiRequestId){
                newApiLoading = false
            }
        }
    }

    function selectNewApiGroup(event: Event) {
        newApiGroup = (event.currentTarget as HTMLSelectElement).value
        newApiModel = ''
        void loadNewApiAccount(newApiGroup)
    }

    function selectNewApiModel(event: Event) {
        if(newApiLoading){
            return
        }
        newApiModel = (event.currentTarget as HTMLSelectElement).value
        if(newApiGroup && newApiModel){
            applyNewApiModelSelection(DBState.db, newApiGroup, newApiModel)
        }
    }

    function continueWithNewApi() {
        if(!newApiGroup || !newApiModel){
            return
        }
        provider = 'new-api'
        applyNewApiModelSelection(DBState.db, newApiGroup, newApiModel)
        step = 5
    }

    onMount(() => {
        if(!newApiBridgeEnabled){
            return
        }
        void loadNewApiAccount()
        const refreshAccount = () => { void loadNewApiAccount() }
        window.addEventListener('focus', refreshAccount)
        return () => window.removeEventListener('focus', refreshAccount)
    })

    function send(){
        switch(step){
            case 1:{
                if(input.length > 0){
                    DBState.db.username = input
                    step = 2
                    input = ''
                }
                break
            }
            case 2:{
                if(['openai','openrouter','horde','later'].includes(input.toLocaleLowerCase())){
                    provider = input.toLocaleLowerCase()
                    step = 3
                    input = ''
                }
                break
            }
            case 4:{
                if(input.length === 0){
                    break
                }
                if(!input.startsWith('sk-')){
                    alertError('Invalid API key')
                    break
                }
                if(provider === 'openai'){
                    DBState.db.openAIKey = input
                }
                if(provider === 'openrouter'){
                    DBState.db.openrouterKey = input
                }
                if(provider === 'claude'){
                    DBState.db.claudeAPIKey = input
                }
                step = 5
                input = ''
                break
            }
        }
    }

    $effect.pre(() => {
        if(step === 10){
            setTimeout(() => {
                DBState.db = setPreset(DBState.db, prebuiltPresets.OAI2)
                DBState.db.textTheme = 'highcontrast'
                updateTextThemeAndCSS()

                switch(chatMemorySelection){
                    case 0:{
                        DBState.db.maxContext = 16000
                        DBState.db.maxResponse = 1000
                        break
                    }
                    case 1:{
                        DBState.db.maxContext = 8000
                        DBState.db.maxResponse = 500
                        break
                    }
                    case 2:{
                        DBState.db.maxContext = 12000
                        DBState.db.maxResponse = 800
                        break
                    }
                    case 3:{
                        DBState.db.maxContext = 100000
                        DBState.db.maxResponse = 1000
                        break
                    }
                }

                if(provider === 'claude'){
                    DBState.db.aiModel = 'claude-sonnet-4-6'
                    DBState.db.subModel = 'claude-sonnet-4-6'
                }

                if(provider === 'openai'){
                    DBState.db.aiModel = 'gpt4o-chatgpt'
                    DBState.db.subModel = 'gpt4o-chatgpt'
                }

                if(provider === 'openrouter'){
                    DBState.db.aiModel = 'openrouter'
                    DBState.db.subModel = 'openrouter'
                    DBState.db.openrouterRequestModel = 'risu/free'
                }
                if(provider === 'horde'){
                    DBState.db.aiModel = 'horde:::auto'
                    DBState.db.subModel = 'horde:::auto'
                }
                if(provider === 'new-api'){
                    applyNewApiModelSelection(DBState.db, newApiGroup, newApiModel)
                }
                if(chatLang !== 0){
                    switch(DBState.db.language){
                        case 'de':{
                            DBState.db.translator = 'de'
                            break
                        }
                        case 'en':{
                            DBState.db.translator = 'en'
                            break
                        }
                        case 'ko':{
                            DBState.db.translator = 'ko'
                            break
                        }
                        case 'cn':{
                            DBState.db.translator = 'zh'
                            break
                        }
                        case 'vi':{
                            DBState.db.translator = 'vi'
                            break
                        }
                        case 'zh-Hant':{
                            DBState.db.translator = 'zh-TW'
                            break
                        }
                    }
                }
                if(chatLang === 1){
                    DBState.db.autoTranslate = true
                    DBState.db.translatorType = 'google'
                    DBState.db.useAutoTranslateInput = true
                }

                DBState.db.didFirstSetup = true
            }, 1000);

            DBState.db.claudeCachingExperimental = true
        }

    });
</script>

<div class="w-full h-full flex justify-center welcome-bg text-textcolor relative bg-gray-900">
    <div class="w-2xl overflow-x-hidden max-w-full min-h-full h-full flex flex-col overflow-y-hidden" class:justify-center={!start}>
        {#if !start}
            <div class="w-full justify-center flex mt-8 logo-animation" onanimationend={() => {
                start = true
            }}>
                <img src="/logo_typo_trans.png" alt="logo" class="w-full max-w-(--breakpoint-sm)  mb-0">
            </div>
        {:else}
            <div class="relative w-full flex-col bg-darkbg grow mt-5 max-w-full p-5 rounded-t-lg overflow-x-hidden flex border-gray-800 border chat-animation overflow-y-auto">
                {#if step === 0}
                    <h2 class="animate-bounce">Choose your language</h2>
                    <div class="flex flex-col items-start ml-2">
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('de')
                            DBState.db.language='de'
                            step = 1
                        }}>• Deutsch</button>
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('en')
                            DBState.db.language='en'
                            step = 1
                        }}>• English</button>
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('ko')
                            DBState.db.language='ko'
                            step = 1
                        }}>• 한국어</button>
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('cn')
                            DBState.db.language='cn'
                            step = 1
                        }}>• 中文</button>
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('zh-Hant')
                            DBState.db.language='zh-Hant'
                            step = 1
                        }}>• 中文(繁體)</button>
                        <button class="hover:text-green-500 transition-colors" onclick={() => {
                            changeLanguage('vi')
                            DBState.db.language='vi'
                            step = 1
                        }}>• Tiếng Việt</button>
                    </div>

                {:else}
                    <Chat name="Airisu" img={airisuStyle} message={newApiBridgeEnabled && siteName ? language.setup.newApiWelcome.replaceAll('{site}', siteName) : language.setup.welcome} isLastMemory={false} />
                    {#if step >= 2}
                        <Chat name={DBState.db.username} message={DBState.db.username} isLastMemory={false} />
                        <Chat name="Airisu" img={airisuStyle} message={newApiBridgeEnabled ? language.setup.newApiDescription.replaceAll('{username}', DBState.db.username) : language.setup.setupLaterMessage.replace('{username}', DBState.db.username)} isLastMemory={false} />
                        {#if newApiBridgeEnabled && newApiAccount?.tavernCredit?.granted_quota > 0}
                            <Chat name="Airisu" img={airisuStyle} message={language.setup.newApiGiftMessage.replaceAll('{amount}', formatNewApiTavernCredit(newApiAccount, true)).replaceAll('{balance}', formatNewApiTavernCredit(newApiAccount))} isLastMemory={false} />
                        {/if}
                    {/if}
                    {#if step === 2}
                        <div class="grid grid-cols-2 gap-4 md:grid-cols-3">
                            <button class="border-l-blue-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1 col-span-2" onclick={() => {
                                step = 3
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.setupMessageOption1}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.setupMessageOption1Desc}</span>
                            </button>
                            <button class="border-l-gray-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                provider = 'later'
                                step = 10
                            }}>
                                <h1 class="text-md font-bold text-start text-gray-500">{language.setup.setupMessageOption2}</h1>
                            </button>
                        </div>
                    {/if}
                    {#if step >= 3}
                        <Chat name={DBState.db.username} message={language.setup.setupMessageOption1} isLastMemory={false} />
                        <Chat name="Airisu" img={airisuStyle} message={newApiBridgeEnabled ? language.setup.newApiChooseModel : language.setup.welcome2.replace('{username}', DBState.db.username)} isLastMemory={false} />
                    {/if}
                    {#if step === 3}
                        {#if newApiBridgeEnabled}
                            <div class="min-w-0 flex flex-col gap-4 py-4 pl-4">
                                <h2 class="text-lg font-semibold text-start leading-7 break-words">{language.setup.newApiTitle.replaceAll('{site}', siteName)}</h2>
                                {#if newApiLoading && !newApiAccount}
                                    <span class="text-textcolor2">{language.setup.newApiLoading}</span>
                                {:else if newApiError && !newApiAccount}
                                    <span class="text-draculared">{language.setup.newApiLoadError}: {newApiError}</span>
                                {:else if newApiAccount}
                                    <dl class={"grid grid-cols-2 gap-x-5 gap-y-3 border-y border-darkborderc py-4 " + (newApiAccount.tavernCredit ? "md:grid-cols-3" : "")}>
                                        <div class={"min-w-0 flex flex-col " + (newApiAccount.tavernCredit ? "col-span-2 md:col-span-1" : "")}>
                                            <dt class="text-sm text-textcolor/65 leading-5">{language.setup.newApiAccount}</dt>
                                            <dd class="mt-auto pt-1 text-base font-medium leading-6 break-words">{newApiAccount.account.display_name || newApiAccount.account.username}</dd>
                                        </div>
                                        <div class="min-w-0 flex flex-col">
                                            <dt class="text-sm text-textcolor/65 leading-5">{language.setup.newApiBalance}</dt>
                                            <dd class="mt-auto pt-1 text-lg font-semibold leading-6 tabular-nums break-words">{formatNewApiBalance(newApiAccount)}</dd>
                                        </div>
                                        {#if newApiAccount.tavernCredit}
                                            <div class="min-w-0 flex flex-col">
                                                <dt class="text-sm text-textcolor/65 leading-5">{language.setup.newApiGiftBalance}</dt>
                                                <dd class="mt-auto pt-1 text-lg font-semibold leading-6 tabular-nums break-words">{formatNewApiTavernCredit(newApiAccount)}</dd>
                                            </div>
                                        {/if}
                                    </dl>
                                    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                                        <div class="min-w-0">
                                            <span class="block text-sm text-textcolor/65 mb-1.5">{language.setup.newApiGroup}</span>
                                            <SelectInput value={newApiGroup} onchange={selectNewApiGroup} size="sm" className="w-full min-w-0 h-11">
                                                {#each Object.keys(newApiAccount.groups) as group}
                                                    <OptionInput value={group}>{group}</OptionInput>
                                                {/each}
                                            </SelectInput>
                                        </div>
                                        <div class="min-w-0">
                                            <span class="block text-sm text-textcolor/65 mb-1.5">{language.setup.newApiModel}</span>
                                            <SelectInput value={newApiModel} onchange={selectNewApiModel} size="sm" className="w-full min-w-0 h-11">
                                                {#each newApiAccount.models as model}
                                                    <OptionInput value={model}>{model}</OptionInput>
                                                {/each}
                                            </SelectInput>
                                            {#if newApiAccount.models.length === 0}
                                                <span class="mt-1 block text-sm text-yellow-300">{language.setup.newApiNoModels}</span>
                                            {/if}
                                        </div>
                                    </div>
                                    {#if newApiError}
                                        <span class="text-draculared text-sm">{newApiError}</span>
                                    {/if}
                                    <div class="flex">
                                        <Button size="sm" className="inline-flex items-center justify-center gap-2 min-h-11 w-full md:w-auto" onclick={continueWithNewApi} disabled={newApiLoading || !newApiGroup || !newApiModel}>
                                            <span>{language.setup.newApiStart}</span>
                                            <ArrowRight size={16}/>
                                        </Button>
                                    </div>
                                {/if}
                            </div>
                        {:else}
                            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                                <button class="border-l-gray-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                    provider = 'claude'
                                    step = 4
                                }}>
                                    <h1 class="text-2xl font-bold text-start">Claude <span class="text-sm p-1 rounded-sm bg-blue-500 text-white">{language.recommended}</span></h1>
                                    <span class="mt-2 text-textcolor2 text-start">{language.setup.claudeDesc}</span>
                                </button>
                                <button class="border-l-blue-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                    provider = 'openai'
                                    step = 4
                                }}>
                                    <h1 class="text-2xl font-bold text-start">OpenAI</h1>
                                    <span class="mt-2 text-textcolor2 text-start">{language.setup.openAIDesc}</span>
                                </button>
                                <button class="border-l-red-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                    provider = 'horde'
                                    step = 10
                                }}>
                                    <h1 class="text-2xl font-bold text-start">Horde</h1>
                                    <span class="mt-2 text-textcolor2 text-start">{language.setup.hordeProvider}</span>
                                </button>
                                <button class="border-l-green-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                    provider = 'openrouter'
                                    step = 4
                                }}>
                                    <h1 class="text-2xl font-bold text-start">OpenRouter</h1>
                                    <span class="mt-2 text-textcolor2 text-start">{language.setup.openRouterProvider}</span>
                                </button>
                            </div>
                        {/if}
                    {/if}
                    {#if step >= 4 && !newApiBridgeEnabled}
                        <Chat name={DBState.db.username} message={provider} isLastMemory={false} />
                        {#if provider === 'openai'}
                            <Chat name="Airisu" img={airisuStyle} message={language.setup.setupOpenAI} isLastMemory={false} />
                        {/if}
                        {#if provider === 'openrouter'}
                            <Chat name="Airisu" img={airisuStyle} message={language.setup.setupOpenRouter} isLastMemory={false} />
                        {/if}
                        {#if provider === 'claude'}
                            {#each language.setup.setupClaudeSteps as step, i}
                                <Chat name="Airisu" img={airisuStyle} message={
                                `![alt text](/welcome/claude/ant_${i}.webp)\n\n${i === 0 ? 'https://console.anthropic.com/login?returnTo=%2F%3F\n\n' : ''}` + step
                            } isLastMemory={false} />
                                
                            {/each}
                        {/if}
                    {/if}
                    {#if step >= 5}
                        <Chat name={DBState.db.username} message={newApiBridgeEnabled ? `${newApiGroup} / ${newApiModel}` : "<HIDDEN>"} isLastMemory={false} />
                        <Chat name="Airisu" img={airisuStyle} message={language.setup.chooseChatType} isLastMemory={false} />
                    {/if}
                    {#if step === 5}
                        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                            <button class="border-l-blue-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatLang = 0
                                step = 6
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseChatTypeOption1}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseChatTypeOption1Desc}</span>
                            </button>
                            <button class="border-l-green-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatLang = 1
                                step = 6
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseChatTypeOption2}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseChatTypeOption2Desc}</span>
                            </button>
                            <button class="border-l-red-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatLang = 2
                                step = 6
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseChatTypeOption3}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseChatTypeOption3Desc}</span>
                            </button>
                        </div>
                    {/if}
                    {#if step >= 6}
                        <Chat name={DBState.db.username} message={
                            language.setup[`chooseChatTypeOption${chatLang+1}`]
                        } isLastMemory={false} />
                        <Chat name="Airisu" img={airisuStyle} message={language.setup.chooseCheapOrMemory} isLastMemory={false} />
                    {/if}
                    {#if step === 6}
                        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                            <button class="border-l-red-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatMemorySelection = 2
                                step = 10
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseCheapOrMemoryOption3} <span class="text-sm p-1 rounded-sm bg-blue-500 text-white">{language.recommended}</span></h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseCheapOrMemoryOption3Desc}</span>
                            </button>
                            <button class="border-l-blue-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatMemorySelection = 0
                                step = 10
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseCheapOrMemoryOption1}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseCheapOrMemoryOption1Desc}</span>
                            </button>
                            <button class="border-l-green-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatMemorySelection = 1
                                step = 10
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseCheapOrMemoryOption2}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseCheapOrMemoryOption2Desc}</span>
                            </button>
                            <button class="border-l-yellow-500 border-l-4 p-6 flex flex-col transition-shadow hover:ring-1" onclick={() => {
                                chatMemorySelection = 3
                                step = 10
                            }}>
                                <h1 class="text-2xl font-bold text-start">{language.setup.chooseCheapOrMemoryOption4}</h1>
                                <span class="mt-2 text-textcolor2 text-start">{language.setup.chooseCheapOrMemoryOption4Desc}</span>
                            </button>
                        </div>
                    {/if}
                    {#if step === 10}
                        <Chat name="Airisu" img={airisuStyle} message={language.setup.allDone} isLastMemory={false} />
                    {/if}
                    {#if !newApiBridgeEnabled || step !== 3}
                    <div class="flex items-stretch mb-2 w-full mt-auto">
                        <textarea class="peer focus:border-textcolor transition-colors outline-hidden text-textcolor p-2 min-w-0 border border-r-0 bg-transparent rounded-md rounded-r-none input-text text-xl grow ml-4 border-darkborderc resize-none overflow-y-hidden overflow-x-hidden max-w-full"
                            bind:value={input}
                            onkeydown={(e) => {
                                if(e.key.toLocaleLowerCase() === "enter" && (!e.shiftKey) && !e.isComposing){
                                    e.preventDefault()
                                    send()
                                }
                            }}
                            style:height={'44px'}
                        ></textarea>
                        <button
                            onclick={send}
                            class="flex justify-center border-y border-r rounded-r-md border-darkborderc items-center text-textcolor p-2 peer-focus:border-textcolor hover:bg-blue-500 hover:text-white transition-colors"
                        >
                            <Send />
                        </button>
                    </div>
                    {/if}
                {/if}
            </div>
        {/if}

    </div>
</div>
<style>
    .welcome-bg{
        background-size: cover;
        position: relative;
    }

    @keyframes darkness {
        from {
            opacity: 0;
        }
        50% {
            opacity: 0.2;
        }
        to {
            opacity: 0;
        }
    }

    .logo-animation{
        animation: logo-animation 3s ease-in-out;
        opacity: 0;
    }
    @keyframes logo-animation {
        from {
            opacity: 0;
        }
        80% {
            opacity: 1;
        }
        to {
            opacity: 0;
        }
    }

    .chat-animation{
        animation: chat-animation 3s ease-in-out;
    }
    @keyframes chat-animation {
        from {
            top: 100vh;
        }
        to {
            top: 0;
        }
    }

</style>
