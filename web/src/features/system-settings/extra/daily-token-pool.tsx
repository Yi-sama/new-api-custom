import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, Eye, GitBranch, RotateCcw, Save } from 'lucide-react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
} from '@/components/ui/input-group'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  getDailyTokenPoolAdmin,
  saveDailyTokenPool,
  type PoolAdminData,
} from '@/features/daily-token-pool/api'
import { getGroups } from '@/features/users/api'

import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'

const schema = z.object({
  enabled: z.boolean(),
  free_usage: z.boolean(),
  title: z.string().trim().min(1).max(40),
  daily_limit: z.number().int().min(1).max(1_000_000_000_000),
  reset_timezone: z
    .string()
    .min(1)
    .refine((value) => {
      try {
        new Intl.DateTimeFormat('en', { timeZone: value })
        return value !== 'Local'
      } catch {
        return false
      }
    }),
  groups: z.array(z.string()),
  channel_ids: z.array(z.number().int().positive()),
  public_visible: z.boolean(),
})

function DailyTokenPoolForm(props: PoolAdminData & { groups: string[] }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: props.config,
  })
  const values = useWatch({ control: form.control })
  const save = useMutation({
    mutationFn: saveDailyTokenPool,
    onSuccess: (config) => {
      form.reset(config)
      void client.invalidateQueries({ queryKey: ['daily-token-pool-admin'] })
      void client.invalidateQueries({ queryKey: ['daily-token-pool'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => toast.error(error.message),
  })
  return (
    <form
      className='flex w-full max-w-4xl min-w-0 flex-col gap-6'
      onSubmit={form.handleSubmit((value) => save.mutate(value))}
      aria-busy={save.isPending}
    >
      <FormNavigationGuard when={form.formState.isDirty} />
      <FieldGroup className='gap-6'>
        <Field orientation='horizontal' className='min-h-14'>
          <FieldLabel htmlFor='pool-enabled'>
            {t('Enable daily shared pool')}
          </FieldLabel>
          <Switch
            id='pool-enabled'
            checked={values.enabled}
            disabled={save.isPending}
            onCheckedChange={(enabled) =>
              form.setValue('enabled', enabled, { shouldDirty: true })
            }
          />
        </Field>
        <Field orientation='horizontal' className='min-h-14'>
          <FieldLabel htmlFor='pool-free-usage'>
            {t('Free shared usage (do not deduct personal balance)')}
          </FieldLabel>
          <Switch
            id='pool-free-usage'
            checked={values.free_usage}
            disabled={save.isPending}
            onCheckedChange={(freeUsage) =>
              form.setValue('free_usage', freeUsage, { shouldDirty: true })
            }
          />
        </Field>
        <Separator />
        <FieldSet className='min-w-0'>
          <FieldLegend className='flex items-center gap-2'>
            <CalendarClock
              className='text-muted-foreground size-4'
              aria-hidden
            />
            {t('Allowance and reset')}
          </FieldLegend>
          <FieldGroup className='grid min-w-0 gap-5 sm:grid-cols-2'>
            <Field data-invalid={!!form.formState.errors.daily_limit}>
              <FieldLabel htmlFor='pool-daily_limit'>
                {t('Daily token allowance')}
              </FieldLabel>
              <InputGroup className='rounded-[8px]'>
                <InputGroupInput
                  id='pool-daily_limit'
                  type='number'
                  min={1}
                  max={1_000_000_000_000}
                  step={1}
                  disabled={save.isPending}
                  aria-invalid={!!form.formState.errors.daily_limit}
                  aria-describedby={
                    form.formState.errors.daily_limit
                      ? 'pool-limit-error'
                      : undefined
                  }
                  {...form.register('daily_limit', { valueAsNumber: true })}
                />
                <InputGroupAddon align='inline-end'>
                  <InputGroupText>Token</InputGroupText>
                </InputGroupAddon>
              </InputGroup>
              {form.formState.errors.daily_limit && (
                <FieldError id='pool-limit-error'>
                  {t('Invalid value')}
                </FieldError>
              )}
            </Field>
            <Field data-invalid={!!form.formState.errors.reset_timezone}>
              <FieldLabel htmlFor='pool-reset_timezone'>
                {t('Reset timezone')}
              </FieldLabel>
              <Input
                id='pool-reset_timezone'
                className='rounded-[8px]'
                disabled={save.isPending}
                aria-invalid={!!form.formState.errors.reset_timezone}
                aria-describedby={
                  form.formState.errors.reset_timezone
                    ? 'pool-timezone-error'
                    : undefined
                }
                {...form.register('reset_timezone')}
              />
              {form.formState.errors.reset_timezone && (
                <FieldError id='pool-timezone-error'>
                  {t('Invalid value')}
                </FieldError>
              )}
            </Field>
          </FieldGroup>
        </FieldSet>
        <Separator />
        <FieldSet className='min-w-0'>
          <FieldLegend className='flex items-center gap-2'>
            <GitBranch className='text-muted-foreground size-4' aria-hidden />
            {t('Routing scope')}
          </FieldLegend>
          <FieldGroup className='grid min-w-0 gap-5 sm:grid-cols-2'>
            <Field className='min-w-0'>
              <FieldLabel htmlFor='pool-groups'>
                {t('Pool routing groups')}
              </FieldLabel>
              <MultiSelect
                id='pool-groups'
                options={[
                  ...new Set([...props.groups, ...(values.groups ?? [])]),
                ]
                  .filter((group) => group !== 'auto')
                  .map((group) => ({ label: group, value: group }))}
                selected={values.groups ?? []}
                placeholder={t('All Groups')}
                className='min-w-0 rounded-[8px]'
                maxVisibleChips={2}
                disabled={save.isPending}
                onChange={(groups) =>
                  form.setValue('groups', groups, { shouldDirty: true })
                }
              />
            </Field>
            <Field className='min-w-0'>
              <FieldLabel htmlFor='pool-channels'>
                {t('Pool channels')}
              </FieldLabel>
              <MultiSelect
                id='pool-channels'
                options={props.channels.map((channel) => ({
                  label: `${channel.name} (#${channel.id})`,
                  value: String(channel.id),
                }))}
                selected={(values.channel_ids ?? []).map(String)}
                placeholder={t('All channels')}
                className='min-w-0 rounded-[8px]'
                maxVisibleChips={2}
                disabled={save.isPending}
                onChange={(channels) =>
                  form.setValue('channel_ids', channels.map(Number), {
                    shouldDirty: true,
                  })
                }
              />
            </Field>
          </FieldGroup>
        </FieldSet>
        <Separator />
        <FieldSet className='min-w-0'>
          <FieldLegend className='flex items-center gap-2'>
            <Eye className='text-muted-foreground size-4' aria-hidden />
            {t('Public display')}
          </FieldLegend>
          <FieldGroup>
            <Field orientation='horizontal'>
              <FieldLabel htmlFor='pool-public'>
                {t('Public pool status')}
              </FieldLabel>
              <Switch
                id='pool-public'
                checked={values.public_visible}
                disabled={save.isPending}
                onCheckedChange={(visible) =>
                  form.setValue('public_visible', visible, {
                    shouldDirty: true,
                  })
                }
              />
            </Field>
            <Field data-invalid={!!form.formState.errors.title}>
              <FieldLabel htmlFor='pool-title'>
                {t('Pool display name')}
              </FieldLabel>
              <Input
                id='pool-title'
                className='rounded-[8px]'
                maxLength={40}
                disabled={save.isPending}
                aria-invalid={!!form.formState.errors.title}
                aria-describedby={
                  form.formState.errors.title ? 'pool-title-error' : undefined
                }
                {...form.register('title')}
              />
              {form.formState.errors.title && (
                <FieldError id='pool-title-error'>
                  {t('Invalid value')}
                </FieldError>
              )}
            </Field>
          </FieldGroup>
        </FieldSet>
      </FieldGroup>
      <Separator />
      <div className='flex flex-wrap gap-2'>
        <Button
          type='submit'
          className='rounded-[8px]'
          disabled={save.isPending || !form.formState.isDirty}
        >
          <Save data-icon='inline-start' />
          {save.isPending ? t('Saving...') : t('Save changes')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className='rounded-[8px]'
          aria-label={t('Reset')}
          title={t('Reset')}
          disabled={save.isPending || !form.formState.isDirty}
          onClick={() => form.reset()}
        >
          <RotateCcw />
        </Button>
      </div>
    </form>
  )
}

export function DailyTokenPoolSettings() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['daily-token-pool-admin'],
    queryFn: getDailyTokenPoolAdmin,
  })
  const groups = useQuery({
    queryKey: ['groups-for-daily-token-pool'],
    queryFn: async () => {
      const response = await getGroups()
      if (!response.success) throw new Error(response.message)
      return response.data
    },
  })
  return (
    <SettingsSection title={t('Daily shared token pool')}>
      {(query.isPending || groups.isPending) && (
        <Skeleton className='h-40 w-full' />
      )}
      {(query.isError || groups.isError) && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to load shared token pool')}</AlertTitle>
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              void query.refetch()
              void groups.refetch()
            }}
          >
            <RotateCcw />
            {t('Retry')}
          </Button>
        </Alert>
      )}
      {query.data && groups.data && (
        <DailyTokenPoolForm
          key={JSON.stringify(query.data.config)}
          {...query.data}
          groups={groups.data}
        />
      )}
    </SettingsSection>
  )
}
