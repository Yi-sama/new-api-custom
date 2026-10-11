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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { isSidebarModuleEnabled } from '@/lib/nav-modules'
import { Route as AuthenticatedRoute } from '@/routes/_authenticated/route'
import { Route as TavernRoute } from '@/routes/_authenticated/tavern'
import { useAuthStore } from '@/stores/auth-store'

vi.mock('@/components/layout', () => ({ AuthenticatedLayout: () => null }))
vi.mock('@/features/tavern', () => ({ Tavern: () => null }))

function createTavernRouter() {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: 'authenticated',
    beforeLoad: (context) =>
      AuthenticatedRoute.options.beforeLoad?.(context as never),
  })
  const tavern = createRoute({
    getParentRoute: () => authenticated,
    path: '/tavern',
    beforeLoad: (context) => TavernRoute.options.beforeLoad?.(context as never),
  })
  const dashboard = createRoute({
    getParentRoute: () => root,
    path: '/dashboard',
  })
  const signIn = createRoute({
    getParentRoute: () => root,
    path: '/sign-in',
  })
  return createRouter({
    routeTree: root.addChildren([
      authenticated.addChildren([tavern]),
      dashboard,
      signIn,
    ]),
    history: createMemoryHistory({ initialEntries: ['/tavern'] }),
  })
}

describe('Tavern route access', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.getState().auth.reset()
  })

  afterEach(() => {
    localStorage.clear()
    useAuthStore.getState().auth.reset()
  })

  it.each([false, true])(
    'keeps signed-in Tavern access when chat.enabled=%s and Playground is disabled',
    async (enabled) => {
      useAuthStore.getState().auth.setBundle({
        user: { id: 7, username: 'route-fixture', role: 1 },
        access_token: 'local-test-only',
        token_type: 'Bearer',
        access_expires_at: 2_000_000_000,
        session: {
          sid: 'local-test-only',
          current: true,
          login_method: 'test',
          ip: '127.0.0.1',
          user_agent: 'test',
          created_at: 1,
          last_active_at: 1,
          expires_at: 2_000_000_000,
        },
      })
      localStorage.setItem(
        'status',
        JSON.stringify({
          SidebarModulesAdmin: JSON.stringify({
            chat: { enabled, playground: false, chat: false },
          }),
        })
      )
      const router = createTavernRouter()

      await router.load()

      expect(router.state.location.pathname).toBe('/tavern')
      expect(isSidebarModuleEnabled('chat', 'playground')).toBe(false)
    }
  )

  it('still redirects anonymous Tavern visitors to sign-in', async () => {
    const router = createTavernRouter()

    await router.load()

    expect(router.state.location.pathname).toBe('/sign-in')
    expect(router.state.location.search).toMatchObject({ redirect: '/tavern' })
  })
})
