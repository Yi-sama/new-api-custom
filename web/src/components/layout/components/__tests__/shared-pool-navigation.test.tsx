import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { useSidebarData } from '@/hooks/use-sidebar-data'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'

describe('Shared pool navigation', () => {
  it('keeps public status in the top navigation but not the ordinary sidebar', () => {
    const client = new QueryClient()
    client.setQueryData(['status'], {})
    const { result } = renderHook(
      () => ({ top: useTopNavLinks(), sidebar: useSidebarData() }),
      {
        wrapper: (props) => (
          <QueryClientProvider client={client}>
            {props.children}
          </QueryClientProvider>
        ),
      }
    )
    expect(
      result.current.top.filter((link) => link.href === '/shared-tokens')
    ).toHaveLength(1)
    expect(
      result.current.sidebar.navGroups.flatMap((group) => group.items)
    ).not.toContainEqual(expect.objectContaining({ url: '/shared-tokens' }))
  })
})
