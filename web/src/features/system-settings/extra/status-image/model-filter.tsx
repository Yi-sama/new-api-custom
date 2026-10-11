import { ChevronDown, Search, X } from 'lucide-react'
import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type { ChannelModelSelection } from './config'
import type { ReportGroup } from './data'

export function StatusImageModelFilter(props: {
  value: ChannelModelSelection | null
  groups: ReportGroup[]
  groupFilter: string
  onChange: (value: ChannelModelSelection | null) => void
  disabled?: boolean
  invalid?: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const [search, setSearch] = useState('')
  const selected = props.value ?? []
  const groups = useMemo(() => {
    const options = new Map<string, Set<string>>()
    for (const group of props.groups) {
      const names = options.get(group.name) ?? new Set<string>()
      group.items.forEach((item) => names.add(item.name))
      options.set(group.name, names)
    }
    for (const item of props.value ?? []) {
      const names = options.get(item.group)
      if (!names) continue
      names.add(item.model)
    }
    return [...options].map(([name, models]) => ({ name, models: [...models] }))
  }, [props.groups, props.value])
  const visibleGroups = new Set(
    props.groupFilter
      .split(/[,\uFF0C\n]/)
      .map((name) => name.trim())
      .filter(Boolean)
  )
  const query = search.trim().toLocaleLowerCase()
  const matches = groups
    .map((group) => ({
      ...group,
      models: group.models.filter((model) =>
        `${group.name} ${model}`.toLocaleLowerCase().includes(query)
      ),
    }))
    .filter((group) => group.models.length)

  return (
    <FieldSet className='min-w-0 gap-3' disabled={props.disabled}>
      <FieldLegend variant='label'>{t('Channel models')}</FieldLegend>
      <ToggleGroup
        aria-label={t('Channel model visibility')}
        value={[props.value === null ? 'all' : 'selected']}
        onValueChange={(value) => {
          if (value.length) props.onChange(value[0] === 'all' ? null : [])
        }}
        variant='outline'
        size='sm'
        disabled={props.disabled}
      >
        <ToggleGroupItem value='all'>{t('All models')}</ToggleGroupItem>
        <ToggleGroupItem value='selected'>
          {t('Selected models only')}
        </ToggleGroupItem>
      </ToggleGroup>
      {props.value !== null && (
        <>
          <Field data-invalid={props.invalid} data-disabled={props.disabled}>
            <InputGroup>
              <InputGroupAddon>
                <Search aria-hidden='true' />
              </InputGroupAddon>
              <InputGroupInput
                aria-label={t('Search groups or models')}
                placeholder={t('Search groups or models')}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                disabled={props.disabled}
                aria-invalid={props.invalid}
                aria-describedby={props.invalid ? `${id}-error` : undefined}
              />
            </InputGroup>
          </Field>
          <div className='flex items-center justify-between gap-2'>
            <span role='status' className='text-muted-foreground text-xs'>
              {t('Selected pairs: {{count}}', { count: selected.length })}
            </span>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    aria-label={t('Clear model selection')}
                    disabled={props.disabled || !selected.length}
                    onClick={() => props.onChange([])}
                  />
                }
              >
                <X aria-hidden='true' data-icon='inline-start' />
              </TooltipTrigger>
              <TooltipContent>{t('Clear model selection')}</TooltipContent>
            </Tooltip>
          </div>
          <div className='flex max-h-80 min-w-0 flex-col overflow-y-auto'>
            {matches.map((group) => (
              <details
                key={group.name}
                open
                className='min-w-0 border-b last:border-b-0'
              >
                <summary className='flex cursor-pointer list-none items-center gap-2 py-2 text-sm [&::-webkit-details-marker]:hidden'>
                  <span className='min-w-0 flex-1 [overflow-wrap:anywhere]'>
                    {group.name}
                  </span>
                  <ChevronDown aria-hidden='true' className='size-4 shrink-0' />
                </summary>
                {visibleGroups.size > 0 && !visibleGroups.has(group.name) && (
                  <p className='text-muted-foreground pb-2 text-xs'>
                    {t('Outside selected groups')}
                  </p>
                )}
                <FieldGroup className='gap-1 pb-2'>
                  {group.models.map((model) => {
                    const checked = selected.some(
                      (item) =>
                        item.group === group.name && item.model === model
                    )
                    const known = props.groups.some(
                      (entry) =>
                        entry.name === group.name &&
                        entry.items.some((item) => item.name === model)
                    )
                    const checkboxId = `${id}-${encodeURIComponent(JSON.stringify([group.name, model]))}`
                    return (
                      <Field
                        key={model}
                        orientation='horizontal'
                        className='min-w-0 py-1'
                      >
                        <Checkbox
                          id={checkboxId}
                          aria-label={`${group.name} / ${model}`}
                          checked={checked}
                          disabled={
                            props.disabled ||
                            (!checked && selected.length >= 500)
                          }
                          onCheckedChange={(value) =>
                            props.onChange(
                              value
                                ? [...selected, { group: group.name, model }]
                                : selected.filter(
                                    (item) =>
                                      item.group !== group.name ||
                                      item.model !== model
                                  )
                            )
                          }
                        />
                        <FieldLabel
                          htmlFor={checkboxId}
                          aria-label={`${group.name} / ${model}`}
                          className='min-w-0 [overflow-wrap:anywhere]'
                        >
                          <span className='flex min-w-0 flex-col gap-1'>
                            {model}
                            {!known && (
                              <span
                                aria-hidden='true'
                                className='text-muted-foreground text-xs'
                              >
                                {t('Not in current data')}
                              </span>
                            )}
                          </span>
                        </FieldLabel>
                      </Field>
                    )
                  })}
                </FieldGroup>
              </details>
            ))}
            {!matches.length && (
              <p role='status' className='text-muted-foreground py-3 text-sm'>
                {t('No matching models')}
              </p>
            )}
          </div>
        </>
      )}
      {props.invalid && (
        <FieldError id={`${id}-error`}>
          {t(
            'Select at most 500 pairs; names must contain 1 to 500 characters'
          )}
        </FieldError>
      )}
    </FieldSet>
  )
}
