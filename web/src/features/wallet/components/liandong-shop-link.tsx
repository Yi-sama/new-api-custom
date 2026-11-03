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
import { ExternalLink, ShoppingBag } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'

const LIANDONG_SHOP_URL = 'https://wzyp.cn/shop/IIIYMQU4'

export function LiandongShopLink() {
  const { t } = useTranslation()

  return (
    <a
      href={LIANDONG_SHOP_URL}
      target='_blank'
      rel='noopener noreferrer'
      className='group border-warning/30 bg-warning/5 hover:border-warning/50 hover:bg-warning/10 focus-visible:border-warning/60 focus-visible:ring-warning/20 flex min-h-16 items-center justify-between gap-3 rounded-lg border px-3 py-2.5 transition-colors focus-visible:ring-3 focus-visible:outline-none'
    >
      <span className='flex min-w-0 items-center gap-2.5'>
        <IconBadge tone='warning' size='sm'>
          <ShoppingBag aria-hidden='true' />
        </IconBadge>
        <span className='min-w-0'>
          <span className='block text-sm leading-5 font-medium break-words'>
            {t('My Liandong Shop')}
          </span>
          <span className='text-muted-foreground block text-xs leading-5 break-words'>
            {t('Explore my shop')}
          </span>
        </span>
      </span>
      <ExternalLink
        aria-hidden='true'
        className='text-muted-foreground size-4 shrink-0 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5'
      />
    </a>
  )
}
