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
import { render } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { Turnstile } from '../turnstile'

afterEach(() => {
  window.turnstile = undefined
})

describe('Turnstile component', () => {
  test('keeps one widget across callback changes and removes it on unmount', () => {
    const renderWidget = vi.fn(
      (_element: HTMLElement, _options: Record<string, unknown>) => 'widget-1'
    )
    const removeWidget = vi.fn()
    const firstOnVerify = vi.fn()
    const latestOnVerify = vi.fn()
    window.turnstile = {
      render: renderWidget,
      remove: removeWidget,
    }

    const rendered = render(
      <Turnstile siteKey='site-key' onVerify={firstOnVerify} />
    )
    const options = renderWidget.mock.calls[0][1] as {
      callback: (token: string) => void
    }

    rendered.rerender(
      <Turnstile siteKey='site-key' onVerify={latestOnVerify} />
    )
    options.callback('verified-token')

    expect(renderWidget).toHaveBeenCalledOnce()
    expect(firstOnVerify).not.toHaveBeenCalled()
    expect(latestOnVerify).toHaveBeenCalledWith('verified-token')

    rendered.unmount()
    expect(removeWidget).toHaveBeenCalledWith('widget-1')
  })
})
