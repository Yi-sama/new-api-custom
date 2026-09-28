import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { AutoModelConfigurator } from '../auto-model-configurator'

async function renderConfigurator(group?: string) {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AutoModelConfigurator group={group} />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      models: ['gpt-cheap', 'gpt-quality'],
      strategy: 'price',
      custom: false,
      allowed_models: ['gpt-cheap', 'gpt-quality', 'claude-fast'],
      default_models: ['gpt-cheap', 'gpt-quality'],
    },
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

describe('Auto model configurator', () => {
  it('shows selected and available models and filters the available list', async () => {
    await renderConfigurator()

    expect((await screen.findAllByText('gpt-cheap')).length).toBeGreaterThan(0)
    expect(screen.getByText('claude-fast')).toBeVisible()

    await userEvent.type(screen.getByPlaceholderText('搜索模型...'), 'claude')

    expect(screen.getByText('claude-fast')).toBeVisible()
    expect(screen.queryAllByText('gpt-quality')).toHaveLength(1)
  })

  it('lets a user choose order strategy, customize models, and save', async () => {
    await renderConfigurator()
    await screen.findAllByText('gpt-cheap')

    await userEvent.click(screen.getByRole('button', { name: '模型顺序优先' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'claude-fast' }))
    await userEvent.click(
      screen.getAllByRole('button', { name: '移除模型' })[0]
    )
    await userEvent.click(screen.getByRole('button', { name: '保存我的 Auto' }))

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/api/user/auto-model', {
        models: ['gpt-quality', 'claude-fast'],
        strategy: 'order',
        inherit_default: false,
      })
    })
  })

  it('loads and saves an independent fixed-group Auto configuration', async () => {
    await renderConfigurator('vip')
    await screen.findAllByText('gpt-cheap')

    expect(api.get).toHaveBeenCalledWith('/api/user/auto-model', {
      params: { group: 'vip' },
    })

    await userEvent.click(screen.getByRole('button', { name: '模型顺序优先' }))
    await userEvent.click(screen.getByRole('button', { name: '保存我的 Auto' }))

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/api/user/auto-model', {
        models: ['gpt-cheap', 'gpt-quality'],
        strategy: 'order',
        inherit_default: true,
        group: 'vip',
      })
    })
  })

  it('applies the site default models immediately from a button', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({
      data: {
        success: true,
        models: ['claude-fast'],
        strategy: 'order',
        custom: true,
        allowed_models: ['gpt-cheap', 'gpt-quality', 'claude-fast'],
        default_models: ['gpt-cheap', 'gpt-quality'],
      },
    })

    await renderConfigurator()
    await screen.findAllByText('claude-fast')

    expect(
      screen.queryByRole('checkbox', { name: '使用站点默认模型列表' })
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '使用站点默认' }))

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/api/user/auto-model', {
        models: ['gpt-cheap', 'gpt-quality'],
        strategy: 'order',
        inherit_default: true,
      })
    })
    expect(
      screen.getByRole('button', { name: '正在使用站点默认' })
    ).toBeDisabled()
  })

  it('shows the full strategy explanation on hover', async () => {
    await renderConfigurator()
    const strategy = await screen.findByRole('button', { name: '低倍率优先' })

    await userEvent.hover(strategy)

    expect(
      await screen.findByText(
        '优先选择预计倍率较低的模型，当前模型失败后再尝试其他可用模型。'
      )
    ).toBeVisible()
  })

  it('shows restore default after changes and restores the default state', async () => {
    await renderConfigurator()
    await screen.findAllByText('gpt-cheap')

    await userEvent.click(screen.getByRole('button', { name: '模型顺序优先' }))
    expect(screen.getByRole('button', { name: '恢复默认' })).toBeVisible()

    await userEvent.click(screen.getByRole('button', { name: '恢复默认' }))
    expect(
      screen.queryByRole('button', { name: '恢复默认' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '低倍率优先' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  it('does not show the site-default action in the user configurator', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({
      data: {
        success: true,
        models: ['gpt-cheap'],
        strategy: 'price',
        custom: true,
        allowed_models: ['gpt-cheap'],
        default_models: ['gpt-cheap'],
      },
    })

    await renderConfigurator()
    await screen.findAllByText('gpt-cheap')

    expect(
      screen.queryByRole('button', { name: '保存为站点默认' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '保存我的 Auto' })).toBeVisible()
  })

  it('shows a retry action when the Auto setting cannot be loaded', async () => {
    vi.mocked(api.get).mockRejectedValueOnce(new Error('network error'))

    await renderConfigurator()

    expect(await screen.findByText('加载失败')).toBeVisible()
    expect(screen.getByRole('button', { name: '重试' })).toBeVisible()
  })

  it('handles empty Auto model arrays returned as null', async () => {
    vi.mocked(api.get).mockResolvedValueOnce({
      data: {
        success: true,
        models: null,
        strategy: 'price',
        custom: false,
        allowed_models: null,
        default_models: null,
      },
    })

    await renderConfigurator()

    expect(await screen.findByText('请至少选择一个 Auto 模型')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: '恢复默认' })
    ).not.toBeInTheDocument()
  })
})
