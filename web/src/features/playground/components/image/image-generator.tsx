import {
  Download,
  FileDown,
  FileUp,
  ImageIcon,
  Loader2,
  Settings2,
  Sparkles,
  Trash2,
} from 'lucide-react'
import { useRef, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import {
  Message,
  MessageContent,
} from '@/components/ai-elements/message'
import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  PromptInputButton,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { sendImageGeneration } from '../../api'
import { buildImageGenerationPayload } from '../../lib/image-generation-payload'
import type {
  GroupOption,
  ImageConversationMessage,
  ImageGenerationConfig,
  ImageGenerationResponse,
  ModelOption,
} from '../../types'
import { getSubmittableInputText } from '../../lib'

const DEFAULT_CONFIG: ImageGenerationConfig = {
  n: 1,
  size: 'default',
  quality: 'default',
  style: 'default',
}

const MAX_MESSAGES = 40

interface ImageGeneratorProps {
  groups: GroupOption[]
  groupValue: string
  isModelLoading?: boolean
  modelValue: string
  models: ModelOption[]
  onGroupChange: (value: string) => void
  onModelChange: (value: string) => void
}

function getImageSource(
  image: NonNullable<ImageGenerationResponse['data']>[number]
): string {
  if (image.url) return image.url
  if (!image.b64_json) return ''
  if (image.b64_json.startsWith('data:')) return image.b64_json
  return `data:image/png;base64,${image.b64_json}`
}

function getErrorMessage(error: unknown): string {
  if (typeof error === 'object' && error !== null) {
    const response = (error as { response?: { data?: unknown } }).response
    const data = response?.data
    if (typeof data === 'object' && data !== null) {
      const nestedError = (data as { error?: { message?: unknown } }).error
      if (typeof nestedError?.message === 'string') return nestedError.message
      if (typeof (data as { message?: unknown }).message === 'string') {
        return (data as { message: string }).message
      }
    }
  }
  return error instanceof Error ? error.message : 'Image generation failed'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function parseImportedMessages(value: unknown): ImageConversationMessage[] {
  if (!Array.isArray(value)) return []

  return value
    .filter((item): item is Record<string, unknown> => {
      if (!isRecord(item)) return false
      if (item.role !== 'user' && item.role !== 'assistant') return false
      return typeof item.createdAt === 'number'
    })
    .map((item) => ({
      id: typeof item.id === 'string' ? item.id : crypto.randomUUID(),
      role: item.role as ImageConversationMessage['role'],
      prompt: typeof item.prompt === 'string' ? item.prompt : undefined,
      images: Array.isArray(item.images)
        ? item.images.filter((image): image is string => typeof image === 'string')
        : undefined,
      revisedPrompts: Array.isArray(item.revisedPrompts)
        ? item.revisedPrompts.filter(
            (prompt): prompt is string => typeof prompt === 'string'
          )
        : undefined,
      error: typeof item.error === 'string' ? item.error : undefined,
      createdAt:
        typeof item.createdAt === 'number' ? item.createdAt : Date.now(),
    }))
    .slice(-MAX_MESSAGES)
}

export function ImageGenerator(props: ImageGeneratorProps) {
  const { t } = useTranslation()
  const [config, setConfig] = useState<ImageGenerationConfig>(DEFAULT_CONFIG)
  const [draft, setDraft] = useState('')
  const [messages, setMessages] = useState<ImageConversationMessage[]>([])
  const [isGenerating, setIsGenerating] = useState(false)
  const importInputRef = useRef<HTMLInputElement>(null)

  const canGenerate =
    Boolean(props.modelValue) &&
    props.models.length > 0 &&
    !props.isModelLoading &&
    !isGenerating

  const updateConfig = <K extends keyof ImageGenerationConfig>(
    key: K,
    value: ImageGenerationConfig[K]
  ) => {
    setConfig((current) => ({ ...current, [key]: value }))
  }

  const appendMessage = (message: ImageConversationMessage) => {
    setMessages((current) => [...current, message].slice(-MAX_MESSAGES))
  }

  const handleGenerate = async (prompt: string) => {
    if (!prompt.trim() || !canGenerate) return

    const cleanPrompt = prompt.trim()
    appendMessage({
      id: crypto.randomUUID(),
      role: 'user',
      prompt: cleanPrompt,
      createdAt: Date.now(),
    })
    setIsGenerating(true)

    try {
      const payload = buildImageGenerationPayload(
        config,
        cleanPrompt,
        props.modelValue,
        props.groupValue
      )
      const response = await sendImageGeneration(payload)
      const images = (response.data ?? [])
        .map(getImageSource)
        .filter((source): source is string => Boolean(source))

      if (images.length === 0) {
        throw new Error('Image generation failed')
      }

      appendMessage({
        id: crypto.randomUUID(),
        role: 'assistant',
        images,
        revisedPrompts: response.data
          ?.map((image) => image.revised_prompt)
          .filter((prompt): prompt is string => Boolean(prompt)),
        createdAt: Date.now(),
      })
    } catch (error: unknown) {
      const message = t(getErrorMessage(error))
      appendMessage({
        id: crypto.randomUUID(),
        role: 'assistant',
        error: message,
        createdAt: Date.now(),
      })
      toast.error(message)
    } finally {
      setIsGenerating(false)
    }
  }

  const handleSubmit = (message: PromptInputMessage) => {
    const text = getSubmittableInputText(message, !canGenerate)
    if (!text) return
    setDraft('')
    void handleGenerate(text)
  }

  const handleClear = () => {
    setMessages([])
    toast.success(t('Image conversation cleared'))
  }

  const handleExport = () => {
    const payload = {
      version: 1,
      exportedAt: new Date().toISOString(),
      model: props.modelValue,
      group: props.groupValue,
      config,
      messages,
    }
    const blob = new Blob([JSON.stringify(payload, null, 2)], {
      type: 'application/json',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `image-conversation-${new Date().toISOString().slice(0, 10)}.json`
    anchor.click()
    URL.revokeObjectURL(url)
    toast.success(t('Image conversation exported'))
  }

  const handleImport = async (
    event: ChangeEvent<HTMLInputElement>
  ) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return

    try {
      const parsed: unknown = JSON.parse(await file.text())
      if (!isRecord(parsed)) throw new Error('Invalid image conversation file')

      const importedMessages = parseImportedMessages(parsed.messages)
      if (importedMessages.length === 0) {
        throw new Error('Invalid image conversation file')
      }

      setMessages(importedMessages)

      const importedConfig = parsed.config
      if (isRecord(importedConfig)) {
        setConfig((current) => ({
          n:
            typeof importedConfig.n === 'number' &&
            Number.isFinite(importedConfig.n)
              ? Math.max(1, Math.min(4, Math.trunc(importedConfig.n)))
              : current.n,
          size:
            typeof importedConfig.size === 'string'
              ? importedConfig.size
              : current.size,
          quality:
            typeof importedConfig.quality === 'string'
              ? importedConfig.quality
              : current.quality,
          style:
            typeof importedConfig.style === 'string'
              ? importedConfig.style
              : current.style,
        }))
      }

      if (
        typeof parsed.model === 'string' &&
        props.models.some((model) => model.value === parsed.model)
      ) {
        props.onModelChange(parsed.model)
      }
      if (
        typeof parsed.group === 'string' &&
        props.groups.some((group) => group.value === parsed.group)
      ) {
        props.onGroupChange(parsed.group)
      }
      toast.success(t('Image conversation imported'))
    } catch (error: unknown) {
      toast.error(t(getErrorMessage(error)))
    }
  }

  return (
    <div className='flex min-h-0 flex-1 flex-col'>
      <div className='mx-auto flex w-full max-w-4xl shrink-0 items-center justify-between gap-3 px-4 py-3'>
        <div className='min-w-0'>
          <h2 className='truncate text-base font-semibold'>
            {t('Image Generation')}
          </h2>
          <p className='text-muted-foreground truncate text-xs'>
            {t('Create images with the selected model and group.')}
          </p>
        </div>
        <div className='flex shrink-0 items-center gap-1'>
          <input
            accept='application/json'
            className='hidden'
            onChange={handleImport}
            ref={importInputRef}
            type='file'
          />
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={t('Import image conversation')}
                  disabled={isGenerating}
                  onClick={() => importInputRef.current?.click()}
                  size='icon-sm'
                  variant='ghost'
                >
                  <FileUp />
                </Button>
              }
            />
            <TooltipContent>{t('Import image conversation')}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={t('Export image conversation')}
                  disabled={messages.length === 0 || isGenerating}
                  onClick={handleExport}
                  size='icon-sm'
                  variant='ghost'
                >
                  <FileDown />
                </Button>
              }
            />
            <TooltipContent>{t('Export image conversation')}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={t('Clear image conversation')}
                  disabled={messages.length === 0 || isGenerating}
                  onClick={handleClear}
                  size='icon-sm'
                  variant='ghost'
                >
                  <Trash2 />
                </Button>
              }
            />
            <TooltipContent>{t('Clear image conversation')}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      <Conversation>
        <ConversationContent className='p-0'>
          <div className='mx-auto w-full max-w-4xl px-4 py-4'>
            {messages.length === 0 ? (
              <div className='flex min-h-[min(520px,calc(100svh-18rem))] flex-col items-center justify-center gap-3 text-center'>
                <ImageIcon className='text-muted-foreground size-9' />
                <div>
                  <p className='text-sm font-medium'>{t('Start a new image')}</p>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {t('Describe the image you want to create')}
                  </p>
                </div>
              </div>
            ) : (
              messages.map((message) => (
                <Message
                  className='py-2.5'
                  from={message.role}
                  key={message.id}
                >
                  <MessageContent
                    className={
                      message.role === 'assistant'
                        ? 'max-w-full bg-transparent p-0'
                        : 'max-w-[85%] whitespace-pre-wrap'
                    }
                    variant={message.role === 'assistant' ? 'flat' : 'contained'}
                  >
                    {message.role === 'user' && message.prompt}
                    {message.role === 'assistant' && message.error && (
                      <p className='text-destructive text-sm'>{message.error}</p>
                    )}
                    {message.role === 'assistant' &&
                      message.images &&
                      message.images.length > 0 && (
                        <div className='grid max-w-2xl gap-3 sm:grid-cols-2'>
                          {message.images.map((source, index) => (
                            <figure
                              className='overflow-hidden rounded-xl border bg-background'
                              key={`${message.id}-${source}`}
                            >
                              <img
                                alt={t('Generated image')}
                                className='aspect-square w-full object-contain'
                                src={source}
                              />
                              <figcaption className='flex justify-end border-t p-1.5'>
                                <Button
                                  render={
                                    <a
                                      download={`generated-image-${index + 1}`}
                                      href={source}
                                      rel='noreferrer'
                                      target='_blank'
                                    />
                                  }
                                  size='sm'
                                  variant='ghost'
                                >
                                  <Download />
                                  {t('Download')}
                                </Button>
                              </figcaption>
                            </figure>
                          ))}
                        </div>
                      )}
                  </MessageContent>
                </Message>
              ))
            )}
          </div>
        </ConversationContent>
        <ConversationScrollButton />
      </Conversation>

      <div className='mx-auto w-full max-w-4xl shrink-0 px-4 pb-4'>
        <PromptInput
          className='relative'
          groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15'
          onSubmit={handleSubmit}
        >
          <PromptInputTextarea
            autoComplete='off'
            autoCorrect='off'
            autoCapitalize='off'
            className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
            disabled={!canGenerate}
            onChange={(event) => setDraft(event.target.value)}
            placeholder={
              props.models.length === 0
                ? t('No image models available for this group')
                : t('Describe the image you want to create')
            }
            spellCheck={false}
            value={draft}
          />
          <PromptInputFooter className='border-border/60 bg-muted/20 border-t px-3 py-2.5'>
            <div className='flex min-w-0 flex-1 flex-wrap items-center gap-1.5'>
              <ModelGroupSelector
                disabled={isGenerating || props.isModelLoading}
                groups={props.groups}
                models={props.models}
                onGroupChange={props.onGroupChange}
                onModelChange={props.onModelChange}
                selectedGroup={props.groupValue}
                selectedModel={props.modelValue}
              />
              <ImageSettings
                config={config}
                disabled={isGenerating}
                onChange={updateConfig}
              />
            </div>
            <Button
              disabled={!canGenerate || !draft.trim()}
              onClick={() => {
                setDraft('')
                void handleGenerate(draft)
              }}
              size='sm'
              type='button'
            >
              {isGenerating ? (
                <Loader2 className='animate-spin' />
              ) : (
                <Sparkles />
              )}
              {isGenerating ? t('Generating...') : t('Generate image')}
            </Button>
          </PromptInputFooter>
        </PromptInput>
      </div>
    </div>
  )
}

function ImageSettings(props: {
  config: ImageGenerationConfig
  disabled: boolean
  onChange: <K extends keyof ImageGenerationConfig>(
    key: K,
    value: ImageGenerationConfig[K]
  ) => void
}) {
  const { t } = useTranslation()

  return (
    <Popover>
      <PopoverTrigger
        render={
          <PromptInputButton
            aria-label={t('Image generation settings')}
            disabled={props.disabled}
            variant='ghost'
          >
            <Settings2 />
          </PromptInputButton>
        }
      />
      <PopoverContent align='start' className='w-72'>
        <div className='grid gap-3'>
          <div>
            <h3 className='text-sm font-medium'>{t('Image generation settings')}</h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Choose how the image should look')}
            </p>
          </div>
          <SettingSelect
            label={t('Image size')}
            value={props.config.size}
            onValueChange={(value) => props.onChange('size', value)}
            options={[
              ['default', t('Default')],
              ['1024x1024', '1024 × 1024'],
              ['1536x1024', '1536 × 1024'],
              ['1024x1536', '1024 × 1536'],
              ['1792x1024', '1792 × 1024'],
              ['1024x1792', '1024 × 1792'],
            ]}
          />
          <SettingSelect
            label={t('Image quality')}
            value={props.config.quality}
            onValueChange={(value) => props.onChange('quality', value)}
            options={[
              ['default', t('Default')],
              ['auto', t('Auto')],
              ['standard', t('Standard')],
              ['hd', 'HD'],
              ['low', t('Low')],
              ['medium', t('Medium')],
              ['high', t('High')],
            ]}
          />
          <SettingSelect
            label={t('Image style')}
            value={props.config.style}
            onValueChange={(value) => props.onChange('style', value)}
            options={[
              ['default', t('Default')],
              ['natural', t('Natural')],
              ['vivid', t('Vivid')],
            ]}
          />
          <label className='grid gap-2 text-sm font-medium'>
            {t('Image count')}
            <Input
              disabled={props.disabled}
              max={4}
              min={1}
              onChange={(event) =>
                props.onChange('n', Number(event.target.value) || 1)
              }
              type='number'
              value={props.config.n}
            />
          </label>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function SettingSelect(props: {
  label: string
  onValueChange: (value: string) => void
  options: Array<[string, string]>
  value: string
}) {
  return (
    <label className='grid gap-2 text-sm font-medium'>
      {props.label}
      <Select
        onValueChange={(value) => {
          if (value !== null) props.onValueChange(value)
        }}
        value={props.value}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {props.options.map(([value, label]) => (
            <SelectItem key={value} value={value}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  )
}
