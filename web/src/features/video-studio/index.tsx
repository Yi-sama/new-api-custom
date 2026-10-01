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
import { RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useAuthStore } from '@/stores/auth-store'

import { VideoStudioAccountBridge } from './account-bridge'
import { createVideoStudioSession, type VideoStudioSession } from './api'

type BoundSession = {
  loadKey: string
  data: VideoStudioSession
}

type BoundError = {
  loadKey: string
  message: string
}

/* oxlint-disable react/iframe-missing-sandbox -- Trusted local app needs scripts and same-origin API/message access. */
export function VideoStudio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const sid = useAuthStore((state) => state.auth.session?.sid)
  const accountKey = user ? `${user.id}:${user.username}:${sid ?? ''}` : ''
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const [session, setSession] = useState<BoundSession | null>(null)
  const [readyKey, setReadyKey] = useState<string | null>(null)
  const [error, setError] = useState<BoundError | null>(null)
  const [attempt, setAttempt] = useState(0)
  const loadKey = `${accountKey}:${attempt}`
  const currentSession = session?.loadKey === loadKey ? session : null
  const currentError = error?.loadKey === loadKey ? error.message : null
  const ready = readyKey === loadKey

  const sendSession = useCallback(
    (target?: Window | null) => {
      if (!currentSession || !target) return
      target.postMessage(
        { type: 'new-api-video-session', session: currentSession.data },
        window.location.origin
      )
    },
    [currentSession]
  )

  useEffect(() => {
    let active = true

    if (!accountKey) return () => undefined

    const expectedUserId = user?.id
    const expectedUsername = user?.username

    void createVideoStudioSession()
      .then((data) => {
        if (!active) return
        if (
          data.user_id !== expectedUserId ||
          data.username !== expectedUsername
        ) {
          setError({ loadKey, message: t('Unable to load Video Studio') })
          return
        }
        setSession({ loadKey, data })
      })
      .catch(() => {
        if (!active) return
        setError({ loadKey, message: t('Unable to load Video Studio') })
      })

    return () => {
      active = false
    }
  }, [accountKey, loadKey, t, user?.id, user?.username])

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (
        event.origin !== window.location.origin ||
        event.source !== iframeRef.current?.contentWindow
      ) {
        return
      }

      if (event.data?.type === 'new-api-video-request-session') {
        sendSession(event.source as Window)
      } else if (event.data?.type === 'new-api-video-ready') {
        setReadyKey(loadKey)
        setError(null)
      } else if (
        event.data?.type === 'new-api-video-error' &&
        typeof event.data.message === 'string'
      ) {
        setReadyKey(null)
        setError({ loadKey, message: event.data.message })
      }
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [loadKey, sendSession])

  return (
    <Main className='bg-background h-dvh w-full'>
      {currentSession && (
        <VideoStudioAccountBridge key={loadKey} frameRef={iframeRef} />
      )}
      <div className='relative min-h-0 flex-1'>
        {currentSession && (
          <iframe
            key={`${accountKey}:${attempt}`}
            ref={iframeRef}
            // Retire the old entry URL, which was served without cache control.
            src='/video-canvas/video?v=2'
            title={t('Video Studio')}
            sandbox='allow-downloads allow-forms allow-modals allow-popups allow-popups-to-escape-sandbox allow-same-origin allow-scripts'
            className='size-full border-0'
            onLoad={(event) => sendSession(event.currentTarget.contentWindow)}
          />
        )}

        {!currentError && !ready && (
          <div
            className='bg-background absolute inset-0 flex items-center justify-center'
            aria-live='polite'
          >
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Spinner />
              <span>{t('Loading Video Studio...')}</span>
            </div>
          </div>
        )}

        {currentError && (
          <div
            className='bg-background absolute inset-0 flex flex-col items-center justify-center gap-3 p-4 text-center'
            role='alert'
          >
            <TriangleAlert
              className='text-destructive size-5'
              aria-hidden='true'
            />
            <p className='max-w-lg text-sm'>{currentError}</p>
            <Button
              type='button'
              size='sm'
              onClick={() => setAttempt((value) => value + 1)}
            >
              <RefreshCw aria-hidden='true' />
              {t('Retry')}
            </Button>
          </div>
        )}
      </div>
    </Main>
  )
}
/* oxlint-enable react/iframe-missing-sandbox */
