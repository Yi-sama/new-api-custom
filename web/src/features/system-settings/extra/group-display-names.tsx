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
import { ArrowDown, ArrowUp, Plus, RotateCcw, Save, Trash2 } from 'lucide-react'
import { useFieldArray, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useSystemOptions } from '../hooks/use-system-options'
import { useUpdateOption } from '../hooks/use-update-option'
import type { OperationsSettings } from '../types'

const storedRow = z.object({ group: z.string(), name: z.string() })
type GroupDisplayNamesValues = { rows: z.infer<typeof storedRow>[] }

function readGroupDisplayNames(value: string): GroupDisplayNamesValues['rows'] {
  const parsed: unknown = JSON.parse(value || '{}')
  if (Array.isArray(parsed)) return z.array(storedRow).parse(parsed)
  return Object.entries(z.record(z.string(), z.string()).parse(parsed)).map(
    ([group, name]) => ({ group, name })
  )
}

export function GroupDisplayNamesSettings(
  props: Pick<OperationsSettings, 'GroupDisplayNames'>
) {
  const { t } = useTranslation()
  const options = useSystemOptions()
  const updateOption = useUpdateOption()
  const schema = z.object({
    rows: z
      .array(
        z.object({
          group: z
            .string()
            .refine((value) => value.trim() !== '', t('Group is required')),
          name: z
            .string()
            .trim()
            .min(1, t('Display name is required'))
            .refine(
              (value) => [...value].length <= 200,
              t('Display names must be at most 200 characters')
            ),
        })
      )
      .superRefine((rows, context) => {
        const groups = new Set<string>()
        const names = new Set<string>()
        rows.forEach((row, index) => {
          if (groups.has(row.group)) {
            context.addIssue({
              code: 'custom',
              path: [index, 'group'],
              message: t('Each group can only appear once'),
            })
          }
          if (names.has(row.name)) {
            context.addIssue({
              code: 'custom',
              path: [index, 'name'],
              message: t('Display names must be unique'),
            })
          }
          groups.add(row.group)
          names.add(row.name)
        })
      }),
  })
  const defaultValues = { rows: readGroupDisplayNames(props.GroupDisplayNames) }
  const form = useForm<GroupDisplayNamesValues>({
    resolver: zodResolver(schema),
    defaultValues,
    mode: 'onChange',
  })
  useResetForm(form, defaultValues)
  const rows = useFieldArray({ control: form.control, name: 'rows' })
  const currentRows = useWatch({ control: form.control, name: 'rows' })
  const storedGroups = options.data?.data.find(
    (option) => option.key === 'UserUsableGroups'
  )?.value
  const storedRatios = options.data?.data.find(
    (option) => option.key === 'GroupRatio'
  )?.value
  const usableGroups = z
    .record(z.string(), z.string())
    .parse(JSON.parse(storedGroups || '{}'))
  const ratios = z
    .record(z.string(), z.number())
    .parse(JSON.parse(storedRatios || '{}'))
  const groupOptions = [...new Set([...Object.keys(usableGroups), 'auto'])].map(
    (group) => {
      let ratio = ''
      if (group === 'auto') ratio = t('Automatic')
      else if (ratios[group] === 0) ratio = t('Free of charge')
      else if (ratios[group] !== undefined) ratio = `\u00d7${ratios[group]}`
      return {
        group,
        label: [group, usableGroups[group], ratio].filter(Boolean).join(' / '),
      }
    }
  )

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
              value: JSON.stringify(values.rows),
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
          <Table
            aria-label={t('Group display names')}
            className='min-w-[620px] table-fixed'
          >
            <TableHeader>
              <TableRow>
                <TableHead className='w-44'>{t('Display name')}</TableHead>
                <TableHead>{t('Group')}</TableHead>
                <TableHead className='w-24'>{t('Order')}</TableHead>
                <TableHead className='w-14'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.fields.map((row, index) => {
                const errors = form.formState.errors.rows?.[index]
                const selectedGroup = currentRows[index]?.group || ''
                const unavailable =
                  selectedGroup !== '' &&
                  !groupOptions.some((option) => option.group === selectedGroup)
                return (
                  <TableRow key={row.id}>
                    <TableCell>
                      <Field
                        data-invalid={!!errors?.name}
                        data-disabled={updateOption.isPending}
                      >
                        <FieldLabel
                          htmlFor={`${row.id}-name`}
                          className='sr-only'
                        >
                          {t('Display name')} {index + 1}
                        </FieldLabel>
                        <Input
                          {...form.register(`rows.${index}.name`)}
                          id={`${row.id}-name`}
                          disabled={updateOption.isPending}
                          aria-invalid={!!errors?.name}
                          aria-describedby={
                            errors?.name ? `${row.id}-name-error` : undefined
                          }
                        />
                        <FieldError
                          id={`${row.id}-name-error`}
                          className='whitespace-normal'
                          errors={[errors?.name]}
                        />
                      </Field>
                    </TableCell>
                    <TableCell>
                      <Field
                        data-invalid={!!errors?.group}
                        data-disabled={updateOption.isPending}
                      >
                        <FieldLabel
                          htmlFor={`${row.id}-group`}
                          className='sr-only'
                        >
                          {t('Group')} {index + 1}
                        </FieldLabel>
                        <NativeSelect
                          {...form.register(`rows.${index}.group`)}
                          id={`${row.id}-group`}
                          className='w-full min-w-0'
                          disabled={updateOption.isPending}
                          aria-invalid={!!errors?.group}
                          aria-describedby={
                            errors?.group ? `${row.id}-group-error` : undefined
                          }
                        >
                          <NativeSelectOption value=''>
                            {t('Select a group')}
                          </NativeSelectOption>
                          {unavailable && (
                            <NativeSelectOption value={selectedGroup}>
                              {selectedGroup} / {t('Unavailable')}
                            </NativeSelectOption>
                          )}
                          {groupOptions.map((option) => (
                            <NativeSelectOption
                              key={option.group}
                              value={option.group}
                            >
                              {option.label}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                        <FieldError
                          id={`${row.id}-group-error`}
                          className='whitespace-normal'
                          errors={[errors?.group]}
                        />
                      </Field>
                    </TableCell>
                    <TableCell>
                      <div className='flex gap-1'>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                size='icon-sm'
                                variant='ghost'
                                aria-label={t('Move up')}
                                disabled={updateOption.isPending || index === 0}
                                onClick={() => rows.move(index, index - 1)}
                              />
                            }
                          >
                            <ArrowUp />
                          </TooltipTrigger>
                          <TooltipContent>{t('Move up')}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                size='icon-sm'
                                variant='ghost'
                                aria-label={t('Move down')}
                                disabled={
                                  updateOption.isPending ||
                                  index === rows.fields.length - 1
                                }
                                onClick={() => rows.move(index, index + 1)}
                              />
                            }
                          >
                            <ArrowDown />
                          </TooltipTrigger>
                          <TooltipContent>{t('Move down')}</TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              size='icon-sm'
                              variant='ghost'
                              aria-label={t('Delete row')}
                              disabled={updateOption.isPending}
                              onClick={() => rows.remove(index)}
                            />
                          }
                        >
                          <Trash2 />
                        </TooltipTrigger>
                        <TooltipContent>{t('Delete row')}</TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </FieldGroup>
        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            disabled={updateOption.isPending}
            onClick={() => rows.append({ group: '', name: '' })}
          >
            <Plus data-icon='inline-start' />
            {t('Add row')}
          </Button>
          {rows.fields.length === 0 && (
            <Button
              type='button'
              variant='outline'
              disabled={updateOption.isPending}
              className='h-auto min-h-8 whitespace-normal'
              onClick={() =>
                rows.replace(
                  ['GPT', 'Claude', 'Gemini', '\u56fd\u6a21'].map((name) => ({
                    group: '',
                    name,
                  }))
                )
              }
            >
              <Plus data-icon='inline-start' />
              {t('GPT / Claude / Gemini / Local models')}
            </Button>
          )}
        </div>
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
