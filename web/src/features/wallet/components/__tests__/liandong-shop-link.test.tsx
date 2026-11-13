/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import zhCN from '@/i18n/locales/zh.json'

import { LiandongShopLink } from '../liandong-shop-link'
import { RechargeFormCard } from '../recharge-form-card'

const rechargeProps = {
  topupInfo: null,
  presetAmounts: [],
  selectedPreset: null,
  onSelectPreset: vi.fn(),
  topupAmount: 0,
  onTopupAmountChange: vi.fn(),
  paymentAmount: 0,
  calculating: false,
  onPaymentMethodSelect: vi.fn(),
  paymentLoading: null,
  redemptionCode: '',
  onRedemptionCodeChange: vi.fn(),
  onRedeem: vi.fn(),
  redeeming: false,
}

describe('Liandong shop link', () => {
  test('opens the shop in a protected new tab', () => {
    render(<LiandongShopLink />)

    const link = screen.getByRole('link', { name: /my liandong shop/i })

    expect(link).toHaveAttribute('href', 'https://wzyp.cn/shop/IIIYMQU4')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })

  test('appears below redemption controls even without a configured purchase URL', () => {
    render(<RechargeFormCard {...rechargeProps} />)

    const input = screen.getByRole('textbox', { name: 'Have a Code?' })
    const link = screen.getByRole('link', { name: /my liandong shop/i })

    expect(
      input.compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  test('can be reached with Tab after the redemption input and button', async () => {
    const user = userEvent.setup()
    render(<RechargeFormCard {...rechargeProps} />)

    await user.tab()
    expect(screen.getByRole('textbox', { name: 'Have a Code?' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Redeem' })).toHaveFocus()
    await user.tab()

    expect(
      screen.getByRole('link', { name: /my liandong shop/i })
    ).toHaveFocus()
  })

  test('preserves a separately configured redemption purchase link', () => {
    render(
      <RechargeFormCard
        {...rechargeProps}
        topupLink='https://example.test/redemption'
      />
    )

    expect(screen.getByRole('link', { name: 'Get one here' })).toHaveAttribute(
      'href',
      'https://example.test/redemption'
    )
    expect(
      screen.getByRole('link', { name: /my liandong shop/i })
    ).toHaveAttribute('href', 'https://wzyp.cn/shop/IIIYMQU4')
  })

  test('updates the shop name and description when the language changes', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'zhCN', resources: { en, zhCN } })
    render(
      <I18nextProvider i18n={i18n}>
        <LiandongShopLink />
      </I18nextProvider>
    )

    const link = screen.getByRole('link')
    expect(link).toHaveTextContent(zhCN.translation['My Liandong Shop'])
    expect(link).toHaveTextContent(zhCN.translation['Explore my shop'])

    await act(async () => {
      await i18n.changeLanguage('en')
    })

    expect(link).toHaveTextContent(en.translation['My Liandong Shop'])
    expect(link).toHaveTextContent(en.translation['Explore my shop'])
  })
})
