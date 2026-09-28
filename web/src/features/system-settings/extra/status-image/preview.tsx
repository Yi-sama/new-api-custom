import { Download } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { useTheme } from '@/context/theme-provider'

import type { ReportKind, StatusImageConfig } from './config'
import type { ChannelReport } from './data'
import { renderStatusImage } from './render'

export function StatusImagePreview(props: {
  config: StatusImageConfig
  kind: ReportKind
  sample: boolean
  report: ChannelReport
}) {
  const { t, i18n } = useTranslation()
  const { resolvedTheme } = useTheme()
  const canvas = useRef<HTMLCanvasElement>(null)
  const host = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    async function draw() {
      setReady(false)
      setError(false)
      try {
        await document.fonts.ready
        const logo = new Image()
        logo.src = '/logo.png'
        await logo.decode()
        if (cancelled || !canvas.current || !host.current) return
        renderStatusImage(canvas.current, {
          config: props.config,
          kind: props.kind,
          sample: props.sample,
          report: props.report,
          theme: getComputedStyle(host.current),
          logo,
          t,
        })
        setReady(true)
      } catch {
        if (!cancelled) setError(true)
      }
    }
    void draw()
    return () => {
      cancelled = true
    }
  }, [
    props.config,
    props.kind,
    props.sample,
    props.report,
    t,
    i18n.language,
    resolvedTheme,
  ])

  const download = () => {
    canvas.current?.toBlob((blob) => {
      if (!blob) {
        toast.error(t('Failed to generate image'))
        return
      }
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${props.kind}-status${props.sample ? '-sample' : ''}.png`
      link.click()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    }, 'image/png')
  }

  return (
    <div className='min-w-0 space-y-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <span className='text-muted-foreground text-sm'>
          Anthropic · PNG · 1200 px
        </span>
        <Button
          type='button'
          variant='outline'
          disabled={!ready}
          onClick={download}
        >
          <Download aria-hidden='true' />
          {t('Download PNG')}
        </Button>
      </div>
      {error && (
        <p role='alert' className='text-destructive text-sm'>
          {t('Failed to generate image')}
        </p>
      )}
      <div
        ref={host}
        data-theme-preset='anthropic'
        className='bg-background overflow-hidden rounded-md border'
      >
        <canvas
          ref={canvas}
          role='img'
          aria-label={t(
            props.kind === 'channel'
              ? 'Channel status image'
              : 'Sub status image'
          )}
          aria-busy={!ready}
          className='block h-auto w-full'
        />
      </div>
    </div>
  )
}
