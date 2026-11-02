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
import { ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  getOfficialPriceComparison,
  isOfficialComparisonModel,
} from '../lib/official-price'
import type { PricingModel } from '../types'

const labels = { input: 'Input', output: 'Output', cache: 'Cached' } as const
const number = new Intl.NumberFormat('en', { maximumSignificantDigits: 6 })

export function OfficialPriceComparison(props: {
  model: PricingModel
  selectedGroup?: string
  priceRate?: number
}) {
  const { t } = useTranslation()
  if (!isOfficialComparisonModel(props.model)) return null
  const comparison = getOfficialPriceComparison(
    props.model,
    props.selectedGroup,
    props.priceRate ?? 1
  )
  if (!comparison) {
    return (
      <div
        className='text-muted-foreground mt-2 text-xs'
        data-official-comparison
        aria-label={t('Official comparison')}
      >
        {t('Official comparison')}: {t('Reference pending verification')}
      </div>
    )
  }
  return (
    <div
      className='mt-2 min-w-0 space-y-1 text-xs'
      data-official-comparison
      aria-label={t('Official comparison')}
    >
      <div
        className='flex flex-wrap items-baseline gap-x-2 gap-y-1'
        title={t('Site selling price divided by official reference price')}
      >
        <span className='text-muted-foreground'>
          {t('Official comparison')}
        </span>
        {comparison.uniformRatio !== undefined ? (
          <strong className='text-primary font-mono tabular-nums'>
            {number.format(comparison.uniformRatio)}x
          </strong>
        ) : (
          comparison.rows.map((row) => (
            <span key={row.type} className='whitespace-nowrap'>
              {t(labels[row.type])}{' '}
              <strong className='text-primary font-mono tabular-nums'>
                {number.format(row.ratio)}x
              </strong>
            </span>
          ))
        )}
        {comparison.pendingCache && (
          <span className='text-muted-foreground'>
            {t('Cached reference pending verification')}
          </span>
        )}
        <a
          href={comparison.source}
          target='_blank'
          rel='noopener noreferrer'
          className='text-muted-foreground hover:text-foreground inline-flex items-center'
          aria-label={t('Official pricing source')}
          title={t('Official pricing source')}
          onClick={(event) => event.stopPropagation()}
        >
          <ExternalLink className='size-3' aria-hidden='true' />
        </a>
      </div>
      <div className='text-muted-foreground flex flex-wrap gap-x-2 gap-y-1'>
        <span>{t('Official reference (CNY / 1M tokens)')}</span>
        {comparison.rows.map((row) => (
          <span key={row.type} className='whitespace-nowrap'>
            {t(labels[row.type])} {number.format(row.officialCny)}
          </span>
        ))}
      </div>
    </div>
  )
}
