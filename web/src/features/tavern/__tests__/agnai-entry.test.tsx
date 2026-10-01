import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { getAgnaiUrl } from '../agnai-url'
import { Tavern } from '../index'

describe('AgnAI standalone entry', () => {
  it('opens the actual upstream app without an iframe or custom chat', async () => {
    const navigate = vi.fn()
    render(<Tavern navigate={navigate} />)
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith(
        'http://localhost:3200/character/list?connect=1'
      )
    )
    expect(screen.getByRole('heading', { name: 'AgnAI' })).toBeInTheDocument()
    expect(document.querySelector('iframe')).toBeNull()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('keeps the hostname to share the existing local login cookie', () => {
    expect(getAgnaiUrl('http://127.0.0.1:3107')).toBe(
      'http://127.0.0.1:3200/character/list?connect=1'
    )
  })

  it('does not send production users to their own localhost', () => {
    expect(() => getAgnaiUrl('https://example.com')).toThrow(
      'AgnAI is only configured for local testing'
    )
  })
})
