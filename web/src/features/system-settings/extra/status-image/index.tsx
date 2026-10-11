import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronDown,
  Maximize2,
  Minimize2,
  RefreshCw,
  RotateCcw,
  Save,
  SlidersHorizontal,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { updateSystemOption } from '../../api'
import { FormNavigationGuard } from '../../components/form-navigation-guard'
import { SettingsSection } from '../../components/settings-section'
import { useSystemOptions } from '../../hooks/use-system-options'
import type { SystemOptionsResponse } from '../../types'
import { getStatusImageOverview } from './api'
import {
  defaultStatusImageConfig,
  STATUS_IMAGE_OPTION,
  statusImageGroupFilter,
  statusImageSchema,
  type ReportKind,
  type StatusImageConfig,
} from './config'
import { channelReport, sampleChannelReport, sampleSubReport } from './data'
import { StatusImageGroupFilter } from './group-filter'
import { StatusImageModelFilter } from './model-filter'
import { StatusImagePreview } from './preview'

export function StatusImageSettings() {
  const { t } = useTranslation()
  const query = useSystemOptions()
  const parsed = useMemo(() => {
    const raw = query.data?.data?.find(
      (item) => item.key === STATUS_IMAGE_OPTION
    )?.value
    try {
      const config = statusImageSchema.parse(
        raw ? JSON.parse(raw) : defaultStatusImageConfig
      )
      return {
        config: {
          ...config,
          channel_group_filter: statusImageGroupFilter(config, 'channel'),
          sub_group_filter: statusImageGroupFilter(config, 'sub'),
          channel_model_filter: config.channel_model_filter ?? null,
          show_sub_details: false,
        },
        stored: !!raw,
      }
    } catch {
      return null
    }
  }, [query.data])

  if (query.isPending) return <p role='status'>{t('Loading...')}</p>
  if (query.isError || !query.data?.success || !parsed) {
    return (
      <div role='alert' className='space-y-3'>
        <p>{t('Failed to load status image settings')}</p>
        <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
      </div>
    )
  }
  return (
    <StatusImageForm
      initial={parsed.config}
      stored={parsed.stored}
      groupRatio={
        query.data.data.find((item) => item.key === 'GroupRatio')?.value
      }
    />
  )
}

function StatusImageForm(props: {
  initial: StatusImageConfig
  stored: boolean
  groupRatio?: string
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [kind, setKind] = useState<ReportKind>('channel')
  const [source, setSource] = useState('live')
  const [stored, setStored] = useState(props.stored)
  const [focused, setFocused] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(true)
  useEffect(() => {
    const desktop = window.matchMedia('(min-width: 1024px)')
    const fold = () => setSettingsOpen(desktop.matches)
    fold()
    desktop.addEventListener('change', fold)
    return () => desktop.removeEventListener('change', fold)
  }, [])
  const form = useForm<StatusImageConfig>({
    resolver: zodResolver(statusImageSchema),
    defaultValues: props.initial,
    mode: 'onChange',
  })
  const draft = useWatch({ control: form.control })
  const modelFilter = useWatch({
    control: form.control,
    name: 'channel_model_filter',
  })
  const config = useMemo(
    () => ({ ...props.initial, ...draft, channel_model_filter: modelFilter }),
    [props.initial, draft, modelFilter]
  )
  const channelLive = useQuery({
    queryKey: ['status-image-overview', 'channel', props.groupRatio],
    queryFn: () => getStatusImageOverview('channel'),
    enabled: source === 'live',
    retry: false,
    refetchOnWindowFocus: 'always',
  })
  const subLive = useQuery({
    queryKey: ['status-image-overview', 'sub'],
    queryFn: () => getStatusImageOverview('sub'),
    enabled: source === 'live' && kind === 'sub',
    retry: false,
    refetchOnWindowFocus: 'always',
  })
  const live = kind === 'channel' ? channelLive : subLive
  const report = useMemo(
    () =>
      channelLive.data?.success && channelLive.data.data.channel
        ? channelReport(channelLive.data.data.channel)
        : null,
    [channelLive.data]
  )
  const subReport =
    subLive.data?.success && subLive.data.data.sub?.status === 'connected'
      ? subLive.data.data.sub
      : null
  const hasReport = kind === 'channel' ? !!report : !!subReport
  const mutation = useMutation({
    mutationFn: async (value: StatusImageConfig) => {
      const response = await updateSystemOption({
        key: STATUS_IMAGE_OPTION,
        value: JSON.stringify(value),
      })
      if (!response.success) {
        throw new Error(
          response.message || t('Failed to save status image settings')
        )
      }
      return value
    },
    onSuccess: (value) => {
      form.reset(value)
      setStored(true)
      queryClient.setQueryData<SystemOptionsResponse>(
        ['system-options'],
        (previous) =>
          previous
            ? {
                ...previous,
                data: [
                  ...previous.data.filter(
                    (item) => item.key !== STATUS_IMAGE_OPTION
                  ),
                  { key: STATUS_IMAGE_OPTION, value: JSON.stringify(value) },
                ],
              }
            : previous
      )
      toast.success(t('Status image settings saved'))
    },
  })
  const textFields = [
    { name: 'site_name', label: t('Report site name'), max: 60 },
    { name: 'channel_title', label: t('Channel image title'), max: 60 },
    { name: 'sub_title', label: t('Sub image title'), max: 60 },
    { name: 'footer', label: t('Image footer'), max: 160 },
  ] as const
  const groupFields = [
    {
      name: 'channel_group_filter',
      label: t('Channel groups'),
      inputLabel: t('Channel groups (comma separated, blank for all)'),
      options:
        source === 'sample'
          ? sampleChannelReport.groups.map((group) => group.name)
          : (channelLive.data?.data?.channel?.groups ?? []).map(
              (group) => group.group
            ),
    },
    {
      name: 'sub_group_filter',
      label: t('Sub groups'),
      inputLabel: t('Sub groups (comma separated, blank for all)'),
      options:
        source === 'sample'
          ? sampleSubReport.groups.map((group) => group.name)
          : (subLive.data?.data?.sub?.groups ?? []).map((group) => group.name),
    },
  ] as const
  const modelGroups =
    source === 'sample' ? sampleChannelReport.groups : (report?.groups ?? [])
  const switches = [
    { name: 'show_success_rate', label: t('24h success rate') },
    { name: 'show_latency', label: t('Latest request duration') },
    { name: 'show_history', label: t('Recent 12 hours') },
    { name: 'show_updated_at', label: t('Last sample') },
  ] as const

  return (
    <SettingsSection
      title={t('Status images')}
      className='[--primary-foreground:var(--background)] [--primary:var(--foreground)] [&_[data-slot=tabs-list]]:rounded-md [&_button:not([role=switch])]:rounded-[5px] [&_input]:rounded-[5px] [&_select]:rounded-[5px]'
    >
      <FormNavigationGuard when={form.formState.isDirty} />
      <TooltipProvider>
        <form
          className='flex min-w-0 flex-col gap-4'
          onSubmit={form.handleSubmit((value) => mutation.mutate(value))}
        >
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <span role='status' className='text-muted-foreground text-xs'>
              {t(
                form.formState.isDirty || !stored
                  ? 'Unsaved changes'
                  : 'No changes'
              )}
            </span>
            <div className='flex items-center gap-2'>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      type='button'
                      variant='outline'
                      size='icon'
                      aria-label={t('Discard changes')}
                      disabled={mutation.isPending || !form.formState.isDirty}
                      onClick={() => form.reset()}
                    />
                  }
                >
                  <RotateCcw aria-hidden='true' />
                </TooltipTrigger>
                <TooltipContent>{t('Discard changes')}</TooltipContent>
              </Tooltip>
              <Button
                type='submit'
                disabled={
                  mutation.isPending || (!form.formState.isDirty && stored)
                }
              >
                <Save aria-hidden='true' />
                {t(mutation.isPending ? 'Saving...' : 'Save changes')}
              </Button>
            </div>
          </div>
          {mutation.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {mutation.error.message}
            </p>
          )}
          <div
            className={cn(
              'grid min-w-0 border-y lg:grid-cols-[272px_minmax(0,1fr)]',
              focused && 'lg:grid-cols-1'
            )}
          >
            <details
              open={settingsOpen}
              onToggle={(event) => setSettingsOpen(event.currentTarget.open)}
              hidden={focused}
              className='min-w-0 border-b lg:border-r lg:border-b-0'
            >
              <summary className='flex cursor-pointer list-none items-center gap-2 px-4 py-3 text-sm lg:hidden [&::-webkit-details-marker]:hidden'>
                <SlidersHorizontal aria-hidden='true' className='size-4' />
                {t('Image settings')}
                <ChevronDown aria-hidden='true' className='ml-auto size-4' />
              </summary>
              <fieldset
                disabled={mutation.isPending}
                className='flex min-w-0 flex-col gap-6 p-4 lg:p-5'
              >
                <section className='flex min-w-0 flex-col gap-3'>
                  <h4 className='text-sm font-medium'>{t('Image text')}</h4>
                  {textFields.map((field) => (
                    <div
                      key={field.name}
                      className='flex min-w-0 flex-col gap-2'
                    >
                      <Label
                        htmlFor={`status-image-${field.name}`}
                        className='text-muted-foreground text-xs'
                      >
                        {field.label}
                      </Label>
                      <Input
                        id={`status-image-${field.name}`}
                        maxLength={field.max}
                        {...form.register(field.name)}
                        aria-invalid={!!form.formState.errors[field.name]}
                        aria-describedby={
                          form.formState.errors[field.name]
                            ? `error-${field.name}`
                            : undefined
                        }
                      />
                      {form.formState.errors[field.name] && (
                        <p
                          id={`error-${field.name}`}
                          className='text-destructive text-xs'
                          role='alert'
                        >
                          {t('Enter 1 to 60 characters')}
                        </p>
                      )}
                    </div>
                  ))}
                </section>
                <section className='flex min-w-0 flex-col gap-3 border-t pt-5'>
                  <h4 className='text-sm font-medium'>{t('Visible groups')}</h4>
                  {groupFields.map((field) => (
                    <Controller
                      key={field.name}
                      control={form.control}
                      name={field.name}
                      render={({ field: control }) => (
                        <StatusImageGroupFilter
                          id={`status-image-${field.name}`}
                          label={field.label}
                          inputLabel={field.inputLabel}
                          value={control.value ?? ''}
                          options={field.options}
                          onChange={control.onChange}
                          disabled={mutation.isPending}
                          invalid={!!form.formState.errors[field.name]}
                        />
                      )}
                    />
                  ))}
                </section>
                <section className='flex min-w-0 flex-col gap-3 border-t pt-5'>
                  <Controller
                    control={form.control}
                    name='channel_model_filter'
                    render={({ field: control }) => (
                      <StatusImageModelFilter
                        value={control.value ?? null}
                        groups={modelGroups}
                        groupFilter={statusImageGroupFilter(config, 'channel')}
                        onChange={control.onChange}
                        disabled={mutation.isPending}
                        invalid={!!form.formState.errors.channel_model_filter}
                      />
                    )}
                  />
                </section>
                <section className='flex min-w-0 flex-col gap-3 border-t pt-5'>
                  <h4 className='text-sm font-medium'>{t('Image content')}</h4>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='status-image-columns'>
                      {t('Channel columns')}
                    </Label>
                    <select
                      id='status-image-columns'
                      className='border-input bg-background h-9 w-full rounded-md border px-3 text-sm'
                      {...form.register('channel_columns', {
                        valueAsNumber: true,
                      })}
                    >
                      <option value={1}>{t('One column')}</option>
                      <option value={2}>{t('Two columns')}</option>
                    </select>
                  </div>
                  <div className='flex flex-col gap-1'>
                    {switches.map((field) => (
                      <Controller
                        key={field.name}
                        control={form.control}
                        name={field.name}
                        render={({ field: control }) => (
                          <div className='flex min-h-10 items-center justify-between gap-3'>
                            <Label htmlFor={`status-image-${field.name}`}>
                              {field.label}
                            </Label>
                            <Switch
                              id={`status-image-${field.name}`}
                              checked={control.value}
                              onCheckedChange={control.onChange}
                            />
                          </div>
                        )}
                      />
                    ))}
                  </div>
                </section>
              </fieldset>
            </details>
            <div className='bg-muted/30 flex min-w-0 flex-col gap-4 p-3 sm:p-5'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <Tabs
                  value={kind}
                  onValueChange={(value) => setKind(value as ReportKind)}
                >
                  <TabsList>
                    <TabsTrigger value='channel'>
                      {t('Channel status')}
                    </TabsTrigger>
                    <TabsTrigger value='sub'>{t('Sub status')}</TabsTrigger>
                  </TabsList>
                </Tabs>
                <div className='flex flex-wrap items-center gap-2'>
                  <select
                    id='status-image-source'
                    aria-label={t('Preview data')}
                    className='border-input bg-background h-9 rounded-md border px-3 text-sm'
                    value={source}
                    onChange={(event) => setSource(event.target.value)}
                  >
                    <option value='sample'>{t('Sample data')}</option>
                    <option value='live'>{t('Current status')}</option>
                  </select>
                  {source === 'live' && (
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            aria-label={t('Refresh')}
                            disabled={live.isFetching}
                            onClick={() => void live.refetch()}
                          />
                        }
                      >
                        <RefreshCw aria-hidden='true' />
                      </TooltipTrigger>
                      <TooltipContent>{t('Refresh')}</TooltipContent>
                    </Tooltip>
                  )}
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          type='button'
                          variant='outline'
                          size='icon'
                          aria-label={t(
                            focused ? 'Exit expanded preview' : 'Expand preview'
                          )}
                          aria-pressed={focused}
                          onClick={() => setFocused(!focused)}
                        />
                      }
                    >
                      {focused ? (
                        <Minimize2 aria-hidden='true' />
                      ) : (
                        <Maximize2 aria-hidden='true' />
                      )}
                    </TooltipTrigger>
                    <TooltipContent>
                      {t(focused ? 'Exit expanded preview' : 'Expand preview')}
                    </TooltipContent>
                  </Tooltip>
                </div>
              </div>
              {source === 'sample' ? (
                <StatusImagePreview
                  key={`${kind}-${statusImageGroupFilter(config, kind)}-${JSON.stringify(modelFilter)}`}
                  config={config}
                  kind={kind}
                  expanded={focused}
                  sample
                  report={sampleChannelReport}
                  subReport={sampleSubReport}
                />
              ) : (
                <>
                  {live.isFetching && <p role='status'>{t('Loading...')}</p>}
                  {!live.isFetching && (live.isError || !hasReport) && (
                    <p role='alert'>
                      {t(
                        kind === 'channel'
                          ? 'Failed to load channel statistics'
                          : 'Failed to load Sub account status'
                      )}
                    </p>
                  )}
                  {!live.isFetching && !live.isError && hasReport && (
                    <StatusImagePreview
                      key={`${kind}-${statusImageGroupFilter(config, kind)}-${JSON.stringify(modelFilter)}`}
                      config={config}
                      kind={kind}
                      expanded={focused}
                      sample={false}
                      report={report ?? { enabled: true, groups: [] }}
                      subReport={subReport ?? undefined}
                    />
                  )}
                </>
              )}
            </div>
          </div>
        </form>
      </TooltipProvider>
    </SettingsSection>
  )
}
