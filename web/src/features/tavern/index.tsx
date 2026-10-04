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
import { ArrowLeft, ExternalLink } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getRisuaiUrl } from './risuai-url'

type TavernProps = {
  navigate?: (url: string) => void
}

export function Tavern(props: TavernProps) {
  const { t } = useTranslation()
  let target: string | undefined
  try {
    target = getRisuaiUrl(window.location.origin)
  } catch {
    target = undefined
  }

  useEffect(() => {
    if (target) {
      const navigate =
        props.navigate ?? ((url: string) => window.location.replace(url))
      navigate(target)
    }
  }, [props.navigate, target])

  return (
    <main className='bg-background flex min-h-dvh items-center justify-center p-6'>
      <div className='flex max-w-sm flex-col items-center gap-4'>
        <h1 className='text-xl font-semibold'>RisuAI</h1>
        {target ? (
          <Button render={<a href={target} />}>
            <ExternalLink aria-hidden='true' />
            {t('Open Tavern')}
          </Button>
        ) : (
          <p role='alert'>
            {t('Tavern is only available in the local test environment.')}
          </p>
        )}
        <Button variant='ghost' render={<a href='/dashboard' />}>
          <ArrowLeft aria-hidden='true' />
          {t('Back to dashboard')}
        </Button>
      </div>
    </main>
  )
}
