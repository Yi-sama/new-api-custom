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
import { Images, RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { useStatus } from '@/hooks/use-status'

import { createImageStudioSession } from './api'

type ImageStudioProps = {
  redirect?: (url: string) => void
}

export function ImageStudio(props: ImageStudioProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [failed, setFailed] = useState(false)
  const [loading, setLoading] = useState(false)
  const started = useRef(false)
  const enabled = status?.image_studio_enabled === true

  const openStudio = useCallback(async () => {
    setFailed(false)
    setLoading(true)
    try {
      const url = await createImageStudioSession()
      const redirect =
        props.redirect ?? ((target: string) => window.location.assign(target))
      redirect(url)
    } catch {
      setFailed(true)
      setLoading(false)
    }
  }, [props.redirect])

  useEffect(() => {
    if (!enabled || started.current) return
    started.current = true
    void openStudio()
  }, [enabled, openStudio])

  if (status && !enabled) {
    return (
      <Main>
        <div className='flex min-h-full flex-1 items-center justify-center p-4 sm:p-6'>
          <Card className='w-full max-w-md'>
            <CardHeader>
              <div className='bg-primary/10 text-primary mb-2 flex size-10 items-center justify-center rounded-lg'>
                <Images aria-hidden='true' />
              </div>
              <CardTitle>{t('Image Studio')}</CardTitle>
            </CardHeader>
            <CardContent>
              <Alert>
                <TriangleAlert aria-hidden='true' />
                <AlertTitle>
                  {t('Open WebUI is not configured yet.')}
                </AlertTitle>
                <AlertDescription>
                  {t('Ask your administrator to connect Open WebUI and an image model.')}
                </AlertDescription>
              </Alert>
            </CardContent>
          </Card>
        </div>
      </Main>
    )
  }

  return (
    <Main>
      <div className='flex min-h-full flex-1 items-center justify-center p-4 sm:p-6'>
        <Card className='w-full max-w-md'>
          <CardHeader>
            <div className='bg-primary/10 text-primary mb-2 flex size-10 items-center justify-center rounded-lg'>
              <Images aria-hidden='true' />
            </div>
            <CardTitle>{t('Image Studio')}</CardTitle>
          </CardHeader>
          <CardContent>
            {failed ? (
              <Alert variant='destructive'>
                <TriangleAlert aria-hidden='true' />
                <AlertTitle>{t('Unable to open Image Studio')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Please try again. If the problem continues, contact your administrator.'
                  )}
                </AlertDescription>
                <div className='col-start-2 mt-3'>
                  <Button type='button' onClick={() => void openStudio()}>
                    <RefreshCw aria-hidden='true' />
                    {t('Retry')}
                  </Button>
                </div>
              </Alert>
            ) : (
              <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                <Spinner />
                <span>
                  {loading
                    ? t('Opening Image Studio...')
                    : t('Preparing Image Studio...')}
                </span>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </Main>
  )
}
