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
import { describe, expect, test } from 'vitest'

import { isWorkbenchReadyMessage } from './protocol'

describe('image workbench message protocol', () => {
  test('accepts readiness only from the same-origin child frame', () => {
    const source = {} as Window
    const message = {
      origin: 'https://dashboard.example.com',
      source,
      data: { type: 'new-api-workbench-ready' },
    } as MessageEvent

    expect(
      isWorkbenchReadyMessage(
        message,
        source,
        'https://dashboard.example.com'
      )
    ).toBe(true)
    expect(
      isWorkbenchReadyMessage(message, source, 'https://evil.example.com')
    ).toBe(false)
    expect(
      isWorkbenchReadyMessage(
        { ...message, source: {} as Window } as MessageEvent,
        source,
        'https://dashboard.example.com'
      )
    ).toBe(false)
  })
})
