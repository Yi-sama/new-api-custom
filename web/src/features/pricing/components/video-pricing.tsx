import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  getConfiguredGroupRatio,
  getDisplayGroupRatio,
} from '../lib/model-helpers'
import type { PricingModel } from '../types'

const amountFormat = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 6,
})

function money(amount: number): string {
  return `¥${amountFormat.format(amount)}`
}

export function VideoPriceSummary(props: {
  model: PricingModel
  selectedGroup?: string
}) {
  const { t } = useTranslation()
  const pricing = props.model.video_pricing
  if (!pricing) return null
  const ratio = getDisplayGroupRatio(props.model, props.selectedGroup)
  return (
    <span className='flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-xs'>
      {pricing.prices.map((entry) => (
        <span
          key={entry.resolution}
          className='text-muted-foreground whitespace-nowrap'
        >
          {entry.resolution}{' '}
          <span className='text-foreground font-mono font-semibold tabular-nums'>
            {money(entry.price_per_second * ratio)}
          </span>
          /{t('second')}
        </span>
      ))}
    </span>
  )
}

export function VideoPricingSection(props: {
  model: PricingModel
  groupRatio: Record<string, number>
}) {
  const { t } = useTranslation()
  const pricing = props.model.video_pricing
  const [seconds, setSeconds] = useState(pricing?.duration_seconds[0] ?? 1)
  const [resolution, setResolution] = useState(
    pricing?.default_resolution ?? ''
  )
  const [group, setGroup] = useState(props.model.enable_groups[0] ?? '')
  if (!pricing || pricing.prices.length === 0) return null
  const ratio = getConfiguredGroupRatio(props.groupRatio, group)
  const selected =
    pricing.prices.find((entry) => entry.resolution === resolution) ??
    pricing.prices[0]
  const selectClass =
    'bg-background h-9 w-full min-w-0 rounded-md border px-2 text-sm'

  return (
    <section className='min-w-0 space-y-4' aria-label={t('Video pricing')}>
      <h2 className='text-sm font-semibold'>{t('Video pricing')}</h2>
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
        <label className='min-w-0 space-y-1.5 text-xs'>
          <span className='text-muted-foreground'>{t('Resolution')}</span>
          <select
            className={selectClass}
            value={selected.resolution}
            onChange={(event) => setResolution(event.target.value)}
          >
            {pricing.prices.map((entry) => (
              <option key={entry.resolution} value={entry.resolution}>
                {entry.resolution || t('Default')}
              </option>
            ))}
          </select>
        </label>
        <label className='min-w-0 space-y-1.5 text-xs'>
          <span className='text-muted-foreground'>
            {t('Duration (seconds)')}
          </span>
          <select
            className={selectClass}
            value={seconds}
            onChange={(event) => setSeconds(Number(event.target.value))}
          >
            {[...pricing.duration_seconds]
              .sort((a, b) => a - b)
              .map((duration) => (
                <option key={duration} value={duration}>
                  {duration}
                </option>
              ))}
          </select>
        </label>
        <label className='min-w-0 space-y-1.5 text-xs'>
          <span className='text-muted-foreground'>{t('Group')}</span>
          <select
            className={selectClass}
            value={group}
            onChange={(event) => setGroup(event.target.value)}
          >
            {props.model.enable_groups.map((name) => (
              <option key={name} value={name}>
                {name} · {getConfiguredGroupRatio(props.groupRatio, name)}x
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className='bg-muted/30 flex min-h-16 flex-wrap items-center justify-between gap-2 rounded-md px-3 py-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground text-xs'>
            {t('Estimated charge')}
          </div>
          <div className='mt-1 font-mono text-xs tabular-nums'>
            {selected.resolution} · {seconds}
            {t('second')} · {ratio}x
          </div>
        </div>
        <output
          aria-label={t('Estimated charge')}
          className='font-mono text-2xl font-semibold tabular-nums'
        >
          {money(selected.price_per_second * seconds * ratio)}
        </output>
      </div>
      <div className='overflow-x-auto'>
        <table className='w-full text-sm'>
          <thead className='text-muted-foreground text-xs'>
            <tr className='border-b'>
              <th scope='col' className='py-2 text-left font-medium'>
                {t('Resolution')}
              </th>
              <th scope='col' className='py-2 text-right font-medium'>
                {t('Price per second (CNY)')}
              </th>
              <th scope='col' className='py-2 text-right font-medium'>
                {t('Cost for {{seconds}} seconds', { seconds })}
              </th>
            </tr>
          </thead>
          <tbody>
            {pricing.prices.map((entry) => (
              <tr key={entry.resolution} className='border-b last:border-0'>
                <th scope='row' className='py-3 text-left font-medium'>
                  {entry.resolution || t('Default')}
                </th>
                <td className='py-3 text-right font-mono tabular-nums'>
                  {money(entry.price_per_second * ratio)}
                </td>
                <td className='py-3 text-right font-mono tabular-nums'>
                  {money(entry.price_per_second * seconds * ratio)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className='text-muted-foreground text-xs leading-relaxed'>
        {t(
          'Charged by resolution and requested duration, not a flat request fee. Failed tasks are refunded.'
        )}
      </p>
    </section>
  )
}
