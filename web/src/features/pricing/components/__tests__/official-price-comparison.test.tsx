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
import { render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import { OfficialPriceComparison } from '../official-price-comparison'

const grok: PricingModel = {
  id: 1,
  model_name: 'grok-4.7',
  quota_type: 0,
  model_ratio: 0.35,
  completion_ratio: 3,
  cache_ratio: 0.25,
  enable_groups: ['Grok', 'free'],
  group_ratio: { Grok: 1.2, free: 0 },
}

async function show(children: React.ReactNode) {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  return render(children, {
    wrapper: (props) => (
      <I18nextProvider i18n={i18n}>{props.children}</I18nextProvider>
    ),
  })
}

describe('Official price comparison display', () => {
  it('updates the comparison when the selected group changes, including free', async () => {
    const view = await show(
      <OfficialPriceComparison model={grok} selectedGroup='Grok' />
    )
    expect(screen.getByText('0.06x')).toBeVisible()
    expect(screen.getByText('Input 14')).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'Official pricing source' })
    ).toHaveAttribute('href', 'https://docs.x.ai/developers/pricing')

    view.rerender(<OfficialPriceComparison model={grok} selectedGroup='free' />)
    expect(screen.getByText('0x')).toBeVisible()
    expect(screen.queryByText('0.06x')).not.toBeInTheDocument()
  })

  it('shows separate Composer input, output and cache multipliers', async () => {
    await show(
      <OfficialPriceComparison
        model={{
          ...grok,
          model_name: 'composer-2.5',
          cache_ratio: 0.15,
          group_ratio: { Grok: 1 },
          enable_groups: ['Grok'],
        }}
      />
    )
    const comparison = screen.getByLabelText('Official comparison')
    expect(
      within(comparison).getByText('0.2x').parentElement
    ).toHaveTextContent('Input')
    expect(
      within(comparison).getByText('0.12x').parentElement
    ).toHaveTextContent('Output')
    expect(
      within(comparison).getByText('0.075x').parentElement
    ).toHaveTextContent('Cached')
  })

  it('marks unknown aliases pending without inventing an official price', async () => {
    await show(
      <OfficialPriceComparison model={{ ...grok, model_name: 'grok-latest' }} />
    )
    expect(screen.getByText(/Reference pending verification/)).toBeVisible()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.queryByText(/0x/)).not.toBeInTheDocument()
  })

  it('limits partial official references to verified input and output prices', async () => {
    await show(
      <OfficialPriceComparison
        model={{
          ...grok,
          model_name: 'qwen3.8-max',
          enable_groups: ['各种国模'],
          group_ratio: { 各种国模: 1 },
          model_ratio: 6.3,
          cache_ratio: 0.125,
        }}
      />
    )
    expect(screen.getAllByText('1.05x')).toHaveLength(2)
    expect(
      screen.getByText('Cached reference pending verification')
    ).toBeVisible()
    expect(screen.queryByText(/^Cached \d/)).not.toBeInTheDocument()
  })

  it('leaves Claude and non-text pricing without comparison labels', async () => {
    const view = await show(
      <OfficialPriceComparison
        model={{ ...grok, enable_groups: ['满血Claude'] }}
      />
    )
    expect(screen.queryByText('Official comparison')).not.toBeInTheDocument()
    view.rerender(
      <OfficialPriceComparison model={{ ...grok, quota_type: 1 }} />
    )
    expect(screen.queryByText('Official comparison')).not.toBeInTheDocument()
  })
})
