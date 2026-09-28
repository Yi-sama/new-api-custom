import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { Footer } from '@/components/layout/components/footer'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ServiceOverview } from '../components/service-overview'

async function renderOverview(
  modelNames: string[],
  options: { privatePricing?: boolean; failedPricing?: boolean } = {}
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['status'], {
    HeaderNavModules: JSON.stringify({
      pricing: { enabled: true, requireAuth: !!options.privatePricing },
    }),
    user_agreement_enabled: true,
    privacy_policy_enabled: true,
  })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: !options.failedPricing,
      data: modelNames.map((model_name) => ({ model_name })),
    },
  })
  const root = createRootRoute()
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => (
      <>
        <ServiceOverview />
        <Footer compact />
      </>
    ),
  })
  const consoleRoute = createRoute({
    getParentRoute: () => root,
    path: '/dashboard',
    component: () => <h1>Console destination</h1>,
  })
  const statusRoute = createRoute({
    getParentRoute: () => root,
    path: '/channels/status',
    component: () => <h1>Status destination</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([home, consoleRoute, statusRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('heading', { name: 'Configured service' })
  return { get }
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.getState().setConfig({
    systemName: 'Configured service',
    footerHtml: '',
  })
})

describe('single-screen service overview', () => {
  it('shows GPT once and groups other catalog families into one coming-soon card', async () => {
    await renderOverview([
      'gpt-4.1',
      'gpt-5',
      'claude-sonnet',
      'gemini-pro',
      'deepseek-chat',
    ])
    expect(await screen.findByText('GPT')).toBeVisible()
    expect(screen.getAllByText('GPT')).toHaveLength(1)
    expect(
      screen.queryByText('Claude', { selector: 'span' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('Gemini')).not.toBeInTheDocument()
    expect(screen.queryByText('DeepSeek')).not.toBeInTheDocument()
    expect(screen.queryByText('V Visa-free')).not.toBeInTheDocument()
    const upcoming = screen.getByText('Other models').closest('li')
    if (!upcoming) throw new Error('Upcoming models card not found')
    expect(within(upcoming).getByText('Coming soon')).toBeVisible()
    expect(within(upcoming).queryByText('API')).not.toBeInTheDocument()
  })

  it('keeps an other-models preview when only GPT is in the catalog', async () => {
    await renderOverview(['gpt-5'])
    expect(await screen.findByText('GPT')).toBeVisible()
    expect(screen.getByText('Other models')).toBeVisible()
    expect(screen.getByText('Coming soon')).toBeVisible()
    expect(
      screen.getByText('Focused on affordable, reliable AI API services')
    ).toBeVisible()
  })

  it.each([{ models: [] }, { models: ['unknown-model'] }])(
    'does not invent model availability for catalog $models',
    async ({ models }) => {
      await renderOverview(models)
      expect(await screen.findByText('Coming soon')).toBeVisible()
      expect(screen.queryByText('API', { exact: true })).not.toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: 'Enter console' })
      ).toBeVisible()
    }
  )

  it('keeps the service entry available when the catalog request fails', async () => {
    await renderOverview(['gpt-5'], { failedPricing: true })
    expect(screen.queryByText('GPT')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Enter console' })).toBeVisible()
  })

  it('does not request a sign-in-only catalog for a guest', async () => {
    const { get } = await renderOverview(['gpt-5'], { privatePricing: true })
    expect(get).not.toHaveBeenCalled()
    expect(screen.queryByText('GPT')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Model Square' })).toHaveAttribute(
      'href',
      '/sign-in?redirect=%2Fpricing'
    )
  })

  it('links to the public catalog when guest access is enabled', async () => {
    await renderOverview(['gpt-5'])
    expect(screen.getByRole('link', { name: 'Model Square' })).toHaveAttribute(
      'href',
      '/pricing'
    )
  })

  it('shows the supplied group QR image, full-size link and group number', async () => {
    await renderOverview([])
    expect(screen.getByText('QQ group number: 1125690897')).toBeVisible()
    expect(screen.getByText('QQ community')).toBeVisible()
    expect(
      screen.getByRole('img', { name: 'QQ group QR code' })
    ).toHaveAttribute('src', '/qq-group-1125690897.png')
    expect(
      screen.getByRole('link', { name: 'View QQ group QR code' })
    ).toHaveAttribute('href', '/qq-group-1125690897.png')
    expect(screen.queryByText('QR code coming soon')).not.toBeInTheDocument()
    expect(
      screen.queryByText('All systems operational')
    ).not.toBeInTheDocument()
  })

  it.each([
    ['button', 'Enter console', 'Console destination'],
    ['link', 'View channel status', 'Status destination'],
  ])('navigates using the %s %s entry', async (role, label, destination) => {
    await renderOverview([])
    await userEvent.click(screen.getByRole(role, { name: label }))
    expect(
      await screen.findByRole('heading', { name: destination })
    ).toBeVisible()
  })

  it('preserves project attribution, configured footer and legal links in compact mode', async () => {
    useSystemConfigStore.getState().setConfig({
      footerHtml: '<span>Configured footer</span>',
    })
    await renderOverview([])
    expect(screen.getByText('Configured footer')).toBeVisible()
    expect(screen.getByRole('link', { name: 'New API' })).toHaveAttribute(
      'href',
      'https://github.com/QuantumNous/new-api'
    )
    expect(
      screen.getByRole('link', { name: 'Privacy Policy' })
    ).toHaveAttribute('href', '/privacy-policy')
    expect(
      screen.getByRole('link', { name: 'User Agreement' })
    ).toHaveAttribute('href', '/user-agreement')
    await waitFor(() =>
      expect(
        screen.queryByText('Powerful API Management Platform')
      ).not.toBeInTheDocument()
    )
  })
})
