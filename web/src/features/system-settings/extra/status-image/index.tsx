import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RotateCcw, Save } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getChannelStatusOverview } from '@/features/channel-status/api'

import { updateSystemOption } from '../../api'
import { FormNavigationGuard } from '../../components/form-navigation-guard'
import { SettingsSection } from '../../components/settings-section'
import { useSystemOptions } from '../../hooks/use-system-options'
import type { SystemOptionsResponse } from '../../types'
import {
  defaultStatusImageConfig,
  STATUS_IMAGE_OPTION,
  statusImageSchema,
  type ReportKind,
  type StatusImageConfig,
} from './config'
import { channelReport, sampleChannelReport } from './data'
import { StatusImagePreview } from './preview'

export function StatusImageSettings() {
  const { t } = useTranslation()
  const query = useSystemOptions()
  const parsed = useMemo(() => {
    const raw = query.data?.data?.find(
      (item) => item.key === STATUS_IMAGE_OPTION
    )?.value
    try {
      return {
        config: statusImageSchema.parse(
          raw ? JSON.parse(raw) : defaultStatusImageConfig
        ),
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
  return <StatusImageForm initial={parsed.config} stored={parsed.stored} />
}

function StatusImageForm(props: {
  initial: StatusImageConfig
  stored: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [kind, setKind] = useState<ReportKind>('channel')
  const [source, setSource] = useState('sample')
  const [stored, setStored] = useState(props.stored)
  const form = useForm<StatusImageConfig>({
    resolver: zodResolver(statusImageSchema),
    defaultValues: props.initial,
    mode: 'onChange',
  })
  const draft = useWatch({ control: form.control })
  const config = useMemo(
    () => ({ ...props.initial, ...draft }),
    [props.initial, draft]
  )
  const live = useQuery({
    queryKey: ['status-image-channel-overview'],
    queryFn: getChannelStatusOverview,
    enabled: kind === 'channel' && source === 'live',
    retry: false,
  })
  const report = useMemo(
    () => (live.data?.success ? channelReport(live.data.data) : null),
    [live.data]
  )
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
    {
      name: 'group_filter',
      label: t('Visible groups (comma separated, blank for all)'),
      max: 500,
    },
  ] as const
  const switches = [
    { name: 'show_success_rate', label: t('24h success rate') },
    { name: 'show_latency', label: t('Latest request duration') },
    { name: 'show_history', label: t('Recent 12 hours') },
    { name: 'show_updated_at', label: t('Last sample') },
    { name: 'show_sub_details', label: t('Sub account excerpts') },
  ] as const

  return (
    <SettingsSection title={t('Status images')}>
      <FormNavigationGuard when={form.formState.isDirty} />
      <div className='grid min-w-0 gap-8 xl:grid-cols-[minmax(260px,320px)_minmax(0,1fr)]'>
        <form
          className='min-w-0 space-y-5'
          onSubmit={form.handleSubmit((value) => mutation.mutate(value))}
        >
          <fieldset disabled={mutation.isPending} className='min-w-0 space-y-5'>
            {textFields.map((field) => (
              <div key={field.name} className='space-y-2'>
                <Label htmlFor={`status-image-${field.name}`}>
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
            <div className='space-y-2'>
              <Label htmlFor='status-image-columns'>
                {t('Channel columns')}
              </Label>
              <select
                id='status-image-columns'
                className='border-input bg-background h-9 w-full rounded-md border px-3 text-sm'
                {...form.register('channel_columns', { valueAsNumber: true })}
              >
                <option value={1}>{t('One column')}</option>
                <option value={2}>{t('Two columns')}</option>
              </select>
            </div>
            <div className='divide-y border-y'>
              {switches.map((field) => (
                <Controller
                  key={field.name}
                  control={form.control}
                  name={field.name}
                  render={({ field: control }) => (
                    <div className='flex items-center justify-between gap-3 py-3'>
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
            <div className='flex flex-wrap gap-2'>
              <Button
                type='submit'
                disabled={
                  mutation.isPending || (!form.formState.isDirty && stored)
                }
              >
                <Save aria-hidden='true' />
                {t(mutation.isPending ? 'Saving...' : 'Save changes')}
              </Button>
              <Button
                type='button'
                variant='outline'
                disabled={!form.formState.isDirty}
                onClick={() => form.reset()}
              >
                <RotateCcw aria-hidden='true' />
                {t('Discard changes')}
              </Button>
            </div>
          </fieldset>
          {mutation.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {mutation.error.message}
            </p>
          )}
        </form>
        <div className='min-w-0 space-y-4'>
          <Tabs
            value={kind}
            onValueChange={(value) => setKind(value as ReportKind)}
          >
            <TabsList>
              <TabsTrigger value='channel'>{t('Channel status')}</TabsTrigger>
              <TabsTrigger value='sub'>Sub2API</TabsTrigger>
            </TabsList>
          </Tabs>
          {kind === 'channel' && (
            <div className='flex flex-wrap items-center gap-3'>
              <Label htmlFor='status-image-source'>{t('Preview data')}</Label>
              <select
                id='status-image-source'
                className='border-input bg-background h-9 rounded-md border px-3 text-sm'
                value={source}
                onChange={(event) => setSource(event.target.value)}
              >
                <option value='sample'>{t('Sample data')}</option>
                <option value='live'>{t('Current channel statistics')}</option>
              </select>
              {source === 'live' && (
                <Button
                  type='button'
                  variant='ghost'
                  disabled={live.isFetching}
                  onClick={() => void live.refetch()}
                >
                  {t('Refresh')}
                </Button>
              )}
            </div>
          )}
          {kind === 'sub' || source === 'sample' ? (
            <StatusImagePreview
              config={config}
              kind={kind}
              sample
              report={sampleChannelReport}
            />
          ) : (
            <>
              {live.isFetching && <p role='status'>{t('Loading...')}</p>}
              {!live.isFetching && (live.isError || !report) && (
                <p role='alert'>{t('Failed to load channel statistics')}</p>
              )}
              {!live.isFetching && !live.isError && report && (
                <StatusImagePreview
                  config={config}
                  kind={kind}
                  sample={false}
                  report={report}
                />
              )}
            </>
          )}
        </div>
      </div>
    </SettingsSection>
  )
}
