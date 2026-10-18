import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RotateCcw, Save } from 'lucide-react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { getGroups } from '@/features/users/api'

import { updateFastModeGroups } from '../api'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import type { OperationsSettings } from '../types'

const fastModeSchema = z.object({ groups: z.array(z.string().min(1)) })
type FastModeValues = z.infer<typeof fastModeSchema>

export function FastModeSettings(
  props: Pick<OperationsSettings, 'FastModeAllowedGroups'>
) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<FastModeValues>({
    resolver: zodResolver(fastModeSchema),
    defaultValues: { groups: props.FastModeAllowedGroups },
  })
  useResetForm(form, { groups: props.FastModeAllowedGroups })
  const selected = useWatch({ control: form.control, name: 'groups' })
  const groupsQuery = useQuery({
    queryKey: ['groups-for-fast-mode'],
    queryFn: async () => {
      const response = await getGroups()
      if (!response.success) {
        throw new Error(response.message || t('Failed to load groups'))
      }
      return response.data
    },
  })
  const groups = [...new Set([...(groupsQuery.data ?? []), ...selected])]
    .filter((group) => group !== 'auto')
    .sort()
  const save = useMutation({
    mutationFn: async (value: FastModeValues) => {
      const response = await updateFastModeGroups(value.groups)
      if (!response.success) {
        throw new Error(response.message || t('Failed to update setting'))
      }
    },
    onSuccess: (_data, value) => {
      form.reset(value)
      void client.invalidateQueries({ queryKey: ['system-options'] })
      void client.invalidateQueries({ queryKey: ['keys'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error: Error) => toast.error(error.message),
  })

  return (
    <SettingsSection title={t('Fast group settings')}>
      <FormNavigationGuard when={form.formState.isDirty} />
      <form
        className='flex flex-col gap-5'
        onSubmit={form.handleSubmit((value) => save.mutate(value))}
        aria-busy={save.isPending}
      >
        <div className='flex flex-wrap items-center justify-between gap-2 text-sm'>
          <span>{t('Fast billing multiplier')}: 1.5x</span>
          <span className='text-muted-foreground'>
            {t('{{count}} groups selected', { count: selected.length })}
          </span>
        </div>
        <FieldSet disabled={save.isPending}>
          <FieldLegend>{t('Groups allowed to use Fast')}</FieldLegend>
          {groupsQuery.isPending && <p role='status'>{t('Loading...')}</p>}
          {groupsQuery.isError && (
            <div role='alert' className='flex flex-wrap items-center gap-3'>
              <p>{t('Failed to load groups')}</p>
              <Button
                type='button'
                variant='outline'
                onClick={() => void groupsQuery.refetch()}
              >
                <RotateCcw />
                {t('Retry')}
              </Button>
            </div>
          )}
          {!groupsQuery.isPending &&
            !groupsQuery.isError &&
            groups.length === 0 && (
              <p className='text-muted-foreground'>
                {t('No groups available')}
              </p>
            )}
          <FieldGroup className='max-h-96 overflow-y-auto'>
            {groups.map((group) => (
              <Field key={group} orientation='horizontal'>
                <Checkbox
                  id={`fast-group-${group}`}
                  checked={selected.includes(group)}
                  disabled={save.isPending}
                  onCheckedChange={(checked) =>
                    form.setValue(
                      'groups',
                      checked
                        ? [...selected, group]
                        : selected.filter((item) => item !== group),
                      { shouldDirty: true }
                    )
                  }
                />
                <FieldLabel
                  htmlFor={`fast-group-${group}`}
                  className='min-w-0 break-all'
                >
                  {group}
                </FieldLabel>
              </Field>
            ))}
          </FieldGroup>
        </FieldSet>
        <div className='flex flex-wrap gap-2'>
          <Button
            type='submit'
            disabled={
              save.isPending ||
              !form.formState.isDirty ||
              !groupsQuery.isSuccess
            }
          >
            <Save />
            {save.isPending ? t('Saving...') : t('Save changes')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={save.isPending || !form.formState.isDirty}
            onClick={() => form.reset()}
          >
            <RotateCcw />
            {t('Reset')}
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}
