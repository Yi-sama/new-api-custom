import { ChevronDown } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'

export function StatusImageGroupFilter(props: {
  id: string
  label: string
  inputLabel: string
  value: string
  options: string[]
  onChange: (value: string) => void
  disabled?: boolean
  invalid?: boolean
}) {
  const { t } = useTranslation()
  const selected = useMemo(
    () =>
      props.value
        .split(/[,\uFF0C\n]/)
        .map((name) => name.trim())
        .filter(Boolean),
    [props.value]
  )
  const options = [...new Set(props.options)]

  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <Label htmlFor={props.id} className='text-muted-foreground text-xs'>
        {props.label}
      </Label>
      <div className='relative min-w-0'>
        <Input
          id={props.id}
          aria-label={props.inputLabel}
          aria-invalid={props.invalid}
          aria-describedby={props.invalid ? `${props.id}-error` : undefined}
          className='pr-10'
          value={props.value}
          placeholder={t('All groups')}
          maxLength={500}
          disabled={props.disabled}
          onChange={(event) => props.onChange(event.target.value)}
        />
        <Popover>
          <PopoverTrigger
            render={
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='absolute top-0 right-0 size-9 rounded-l-none'
                aria-label={props.label}
                disabled={props.disabled}
              />
            }
          >
            <ChevronDown aria-hidden='true' />
          </PopoverTrigger>
          <PopoverContent
            align='end'
            className='max-w-[calc(100vw-32px)] [--primary-foreground:var(--background)] [--primary:var(--foreground)]'
          >
            <PopoverTitle>{props.label}</PopoverTitle>
            <Label className='flex min-h-9 items-center gap-3 border-b pb-2'>
              <Checkbox
                checked={selected.length === 0}
                onCheckedChange={() => props.onChange('')}
                disabled={props.disabled}
              />
              {t('All groups')}
            </Label>
            <div className='flex max-h-64 flex-col gap-1 overflow-y-auto'>
              {options.map((name) => (
                <Label
                  key={name}
                  className='flex min-h-9 items-center gap-3 py-1 [overflow-wrap:anywhere]'
                >
                  <Checkbox
                    checked={selected.includes(name)}
                    disabled={props.disabled}
                    onCheckedChange={(checked) => {
                      const next = checked
                        ? [...new Set([...selected, name])]
                        : selected.filter((value) => value !== name)
                      props.onChange(next.join(','))
                    }}
                  />
                  <span className='min-w-0'>{name}</span>
                </Label>
              ))}
            </div>
          </PopoverContent>
        </Popover>
      </div>
      {props.invalid && (
        <p
          id={`${props.id}-error`}
          role='alert'
          className='text-destructive text-xs'
        >
          {t('Use at most 500 characters')}
        </p>
      )}
    </div>
  )
}
