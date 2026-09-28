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
import OpenAI from '@lobehub/icons/es/OpenAI/components/Mono'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Activity, ArrowRight, Layers, MessageCircle, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { getPricing } from '@/features/pricing/api'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'

export function ServiceOverview() {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const navLinks = useTopNavLinks()
  const pricingLink = navLinks.find((link) => link.href === '/pricing')
  const canViewModels = !!pricingLink && !pricingLink.requiresAuth
  const { data } = useQuery({
    queryKey: ['pricing'],
    queryFn: getPricing,
    enabled: canViewModels,
    staleTime: 5 * 60 * 1000,
  })
  const hasGpt =
    canViewModels &&
    data?.success &&
    data.data.some((model) => /gpt/i.test(model.model_name))

  return (
    <main className='home-overview' id='main-content'>
      <section className='home-intro' aria-labelledby='home-title'>
        <h1 id='home-title' className='home-title'>
          {systemName}
        </h1>
        <p className='home-description'>
          {t('Focused on affordable, reliable AI API services')}
        </p>
        <div className='home-actions'>
          <Button
            className='home-console h-11 gap-3 px-5 text-sm'
            render={<Link to='/dashboard' />}
          >
            {t('Enter console')}
            <ArrowRight className='size-4' aria-hidden='true' />
          </Button>
          <Link to='/channels/status' className='home-status'>
            <Activity className='size-4' aria-hidden='true' />
            {t('View channel status')}
          </Link>
        </div>
      </section>

      <div className='home-details'>
        <section className='home-models' aria-labelledby='home-models-title'>
          <div className='home-section-heading'>
            <span className='home-section-icon'>
              <Layers size={18} aria-hidden='true' />
            </span>
            <h2 id='home-models-title'>{t('Model access')}</h2>
          </div>
          <p className='home-models-description'>
            {t('Leading models. One API.')}
          </p>
          <ul className='home-model-list' aria-label={t('Model access')}>
            {hasGpt && (
              <li className='home-model' data-family='GPT'>
                <OpenAI size={32} aria-hidden='true' />
                <span>GPT</span>
                <span className='home-model-caption'>API</span>
              </li>
            )}
            <li className='home-model home-model-upcoming'>
              <Plus size={32} aria-hidden='true' />
              <span>{t('Other models')}</span>
              <span className='home-model-caption'>{t('Coming soon')}</span>
            </li>
          </ul>
          {pricingLink && (
            <Link
              to={pricingLink.requiresAuth ? '/sign-in' : '/pricing'}
              search={
                pricingLink.requiresAuth ? { redirect: '/pricing' } : undefined
              }
              className='home-catalog'
            >
              {t('Model Square')}
              <ArrowRight className='size-3.5' aria-hidden='true' />
            </Link>
          )}
        </section>

        <aside
          className='home-community'
          aria-labelledby='home-community-title'
        >
          <div className='home-section-heading'>
            <span className='home-section-icon'>
              <MessageCircle size={18} aria-hidden='true' />
            </span>
            <h2 id='home-community-title'>{t('QQ community')}</h2>
          </div>
          <div className='home-community-body'>
            <a
              className='home-qr'
              href='/qq-group-1125690897.png'
              target='_blank'
              rel='noopener noreferrer'
              aria-label={t('View QQ group QR code')}
              title={t('View QQ group QR code')}
            >
              <img
                src='/qq-group-1125690897.png'
                alt={t('QQ group QR code')}
                width={504}
                height={502}
              />
            </a>
            <div className='home-community-copy'>
              <p>{t('Stay connected')}</p>
              <span>{t('QQ group number')}: 1125690897</span>
            </div>
          </div>
        </aside>
      </div>
    </main>
  )
}
