import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { ApiKeyGroupCell } from '../api-key-group-cell'

describe('channel ratio range', () => {
  it('shows the range for a non-auto group', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
    render(
      <I18nextProvider i18n={i18n}>
        <TooltipProvider>
          <ApiKeyGroupCell
            group='default'
            ratio='0.15-0.18'
            crossGroupRetry={false}
            shouldReduceMotion
          />
        </TooltipProvider>
      </I18nextProvider>
    )
    expect(screen.getByText('0.15-0.18x')).toBeInTheDocument()
  })
})
