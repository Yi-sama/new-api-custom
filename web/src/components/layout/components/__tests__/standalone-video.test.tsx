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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { ThemeCustomizationProvider } from '@/context/theme-customization-provider'
import { ThemeProvider } from '@/context/theme-provider'

import { AuthenticatedLayout } from '../authenticated-layout'

vi.mock('@/lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api')>()),
  getStatus: vi.fn().mockResolvedValue({}),
}))

describe('Standalone authenticated workspace layout', () => {
  beforeEach(() => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  })

  it.each([
    ['/video-studio', false],
    ['/video-studio/', false],
    ['/tavern', false],
    ['/tavern/', false],
    ['/dashboard', true],
  ] as const)(
    'renders %s with the expected console chrome',
    async (path, hasConsole) => {
      const root = createRootRoute({ component: AuthenticatedLayout })
      const video = createRoute({
        getParentRoute: () => root,
        path: path.replace(/\/$/, ''),
        component: () => <p>Page content</p>,
      })
      const router = createRouter({
        routeTree: root.addChildren([video]),
        history: createMemoryHistory({ initialEntries: [path] }),
      })
      render(
        <QueryClientProvider client={new QueryClient()}>
          <ThemeProvider>
            <DirectionProvider>
              <ThemeCustomizationProvider>
                <RouterProvider router={router} />
              </ThemeCustomizationProvider>
            </DirectionProvider>
          </ThemeProvider>
        </QueryClientProvider>
      )

      expect(await screen.findByText('Page content')).toBeInTheDocument()
      expect(Boolean(screen.queryByRole('banner'))).toBe(hasConsole)
      expect(Boolean(document.querySelector('[data-slot="sidebar"]'))).toBe(
        hasConsole
      )
    }
  )
})
