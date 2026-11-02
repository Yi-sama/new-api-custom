/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { RotateCcw, Save } from 'lucide-react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'

import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  formatJsonForTextarea,
  normalizeJsonString,
  validateJsonString,
} from '../models/utils'
import type { OperationsSettings } from '../types'

type GroupDisplayNamesValues = Pick<OperationsSettings, 'GroupDisplayNames'>

export function GroupDisplayNamesSettings(props: GroupDisplayNamesValues) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = z.object({
    GroupDisplayNames: z.string().refine(
      (value) =>
        validateJsonString(value, {
          allowEmpty: false,
          predicate: (parsed) =>
            parsed !== null &&
            typeof parsed === 'object' &&
            !Array.isArray(parsed) &&
            Object.entries(parsed).every(
              ([group, name]) => group.trim() !== '' && typeof name === 'string'
            ),
        }).valid,
      t('Expected a JSON object mapping group identifiers to display names')
    ),
  })
  const defaultValues = {
    GroupDisplayNames: formatJsonForTextarea(props.GroupDisplayNames),
  }
  const form = useForm<GroupDisplayNamesValues>({
    resolver: zodResolver(schema),
    defaultValues,
    mode: 'onChange',
  })
  useResetForm(form, defaultValues)

  return (
    <SettingsSection title={t('Group display names')}>
      <FormNavigationGuard when={form.formState.isDirty} />
      <form
        className='flex min-w-0 flex-col gap-5'
        aria-busy={updateOption.isPending}
        onSubmit={form.handleSubmit((values) =>
          updateOption.mutate(
            {
              key: 'GroupDisplayNames',
              value: normalizeJsonString(values.GroupDisplayNames),
            },
            {
              onSuccess: (response) => {
                if (response.success) form.reset(values)
              },
            }
          )
        )}
      >
        <FieldGroup>
          <Controller
            control={form.control}
            name='GroupDisplayNames'
            render={({ field, fieldState }) => (
              <Field
                data-invalid={fieldState.invalid}
                data-disabled={updateOption.isPending}
                className='min-w-0'
              >
                <FieldLabel htmlFor={field.name}>
                  {t('Group display names')}
                </FieldLabel>
                <JsonCodeEditor
                  id={field.name}
                  name={field.name}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  textareaRef={field.ref}
                  ariaLabel={t('Group display names')}
                  aria-invalid={fieldState.invalid}
                  aria-describedby={
                    fieldState.invalid ? `${field.name}-error` : undefined
                  }
                  disabled={updateOption.isPending}
                />
                <FieldError
                  id={`${field.name}-error`}
                  errors={[fieldState.error]}
                />
              </Field>
            )}
          />
        </FieldGroup>
        <div className='flex flex-wrap gap-2'>
          <Button
            type='submit'
            disabled={updateOption.isPending || !form.formState.isDirty}
          >
            <Save data-icon='inline-start' />
            {updateOption.isPending ? t('Saving...') : t('Save changes')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={updateOption.isPending || !form.formState.isDirty}
            onClick={() => form.reset()}
          >
            <RotateCcw data-icon='inline-start' />
            {t('Reset')}
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}
