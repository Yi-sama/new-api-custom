import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { AutoModelSettings } from '../auto-model'

async function renderSettings() {
  const i18n = createInstance()
  await i18n.init({ lng: 'zh', resources: { zh: { translation: {} } } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AutoModelSettings AutoModelDefaultModels={[]} />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['gpt-cheap', 'gpt-quality'] },
  })
  vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, message: '' },
  })
})

describe('Auto model administrator settings', () => {
  it('saves ordered site-default models', async () => {
    await renderSettings()

    expect((await screen.findAllByText('gpt-cheap')).length).toBeGreaterThan(0)
    await userEvent.click(
      screen.getAllByRole('button', { name: '移除模型' })[0]
    )
    await userEvent.click(
      screen.getByRole('button', { name: '保存 Auto 默认设置' })
    )

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/api/option/', {
        key: 'AutoModelDefaultModels',
        value: JSON.stringify(['gpt-quality']),
      })
    })
  })
})
