import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { FastModeBadge } from '../fast-mode-badge'

describe('usage log Fast mode', () => {
  test('shows the recorded multiplier even when trusted OAuth returned default tier', () => {
    render(
      <FastModeBadge
        other={{
          fast_mode: true,
          fast_mode_ratio: 1.5,
          service_tier: 'default',
          fast_mode_billing_protocol: 'codex-oauth-priority-v1',
        }}
      />
    )

    expect(screen.getByText('Fast 1.5x')).toBeVisible()
  })

  test('shows standard billing when a Fast request was settled at 1x', () => {
    render(<FastModeBadge other={{ fast_mode: true, fast_mode_ratio: 1 }} />)

    expect(screen.getByText('Fast · standard 1x')).toBeVisible()
    expect(screen.queryByText('Fast 1.5x')).not.toBeInTheDocument()
  })

  test('does not invent a multiplier when a Fast log has no recorded ratio', () => {
    render(<FastModeBadge other={{ fast_mode: true }} />)

    expect(screen.getByText('Fast requested')).toBeVisible()
    expect(screen.queryByText('Fast 1.5x')).not.toBeInTheDocument()
  })

  test.each([null, {}, { service_tier: 'priority' }, { fast_mode: false }])(
    'does not label an ordinary or historical log as Fast (%j)',
    (other) => {
      render(<FastModeBadge other={other} />)
      expect(screen.queryByText(/Fast/)).not.toBeInTheDocument()
    }
  )

  test('does not assume the site default when a different multiplier was recorded', () => {
    render(<FastModeBadge other={{ fast_mode: true, fast_mode_ratio: 2 }} />)

    expect(screen.getByText('Fast 2x')).toBeVisible()
  })
})
