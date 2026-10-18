import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'

import { filterByQuotaType } from '../../lib/filters'
import type { PricingModel } from '../../types'
import { ModelBillingModeBadge } from '../model-billing-mode-badge'
import { VideoPricingSection, VideoPriceSummary } from '../video-pricing'

const model: PricingModel = {
  id: 1,
  model_name: 'grok-imagine-video',
  quota_type: 1,
  model_price: 0.25,
  model_ratio: 0,
  completion_ratio: 0,
  enable_groups: ['video'],
  group_ratio: { video: 1 },
  video_pricing: {
    currency: 'CNY',
    unit: 'second',
    duration_seconds: [8, 6, 10],
    default_resolution: '480p',
    prices: [
      { resolution: '480p', price_per_second: 0.25 },
      { resolution: '720p', price_per_second: 0.35 },
    ],
  },
}

async function show(children: React.ReactNode) {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  render(<I18nextProvider i18n={i18n}>{children}</I18nextProvider>)
}

describe('Video pricing', () => {
  it('labels variable video pricing per second instead of per request', async () => {
    await show(
      <>
        <ModelBillingModeBadge model={model} />
        <VideoPriceSummary model={model} />
      </>
    )
    expect(screen.getByText('Per Second')).toBeVisible()
    expect(screen.queryByText('Per Request')).not.toBeInTheDocument()
    expect(screen.getByText(/480p/)).toHaveTextContent('¥0.25')
    expect(screen.getByText(/720p/)).toHaveTextContent('¥0.35')
  })

  it('estimates distinct charges for resolution and duration changes at 1x', async () => {
    await show(<VideoPricingSection model={model} groupRatio={{ video: 1 }} />)
    expect(screen.getByLabelText('Estimated charge')).toHaveTextContent('¥2')
    const row720 = screen.getByRole('row', { name: /720p/ })
    expect(within(row720).getByText('¥2.8')).toBeVisible()
    await userEvent.selectOptions(screen.getByLabelText('Resolution'), '720p')
    expect(screen.getByLabelText('Estimated charge')).toHaveTextContent('¥2.8')
    await userEvent.selectOptions(
      screen.getByLabelText('Duration (seconds)'),
      '10'
    )
    expect(screen.getByLabelText('Estimated charge')).toHaveTextContent('¥3.5')
    expect(screen.queryByText('Per request')).not.toBeInTheDocument()
  })

  it('applies the selected group multiplier to both the table and estimate', async () => {
    const multiGroup = { ...model, enable_groups: ['video', 'premium'] }
    await show(
      <VideoPricingSection
        model={multiGroup}
        groupRatio={{ video: 1, premium: 2 }}
      />
    )
    await userEvent.selectOptions(screen.getByLabelText('Group'), 'premium')
    expect(screen.getByLabelText('Estimated charge')).toHaveTextContent('¥4')
    expect(
      within(screen.getByRole('row', { name: /720p/ })).getByText('¥5.6')
    ).toBeVisible()
  })

  it('keeps non-video fixed pricing marked per request', async () => {
    await show(
      <ModelBillingModeBadge model={{ ...model, video_pricing: undefined }} />
    )
    expect(screen.getByText('Per Request')).toBeVisible()
  })

  it('separates per-second videos from the per-request filter', () => {
    const fixed = {
      ...model,
      model_name: 'fixed-image',
      video_pricing: undefined,
    }
    expect(filterByQuotaType([model, fixed], 'second')).toEqual([model])
    expect(filterByQuotaType([model, fixed], 'request')).toEqual([fixed])
  })
})
