import { ChevronLeft, ChevronRight, Download } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'

import {
  statusImageGroupFilter,
  type ReportKind,
  type StatusImageConfig,
} from './config'
import { channelReportPages, type ChannelReport, type SubReport } from './data'
import { renderStatusImage } from './render'

export function StatusImagePreview(props: {
  config: StatusImageConfig
  kind: ReportKind
  sample: boolean
  expanded?: boolean
  report: ChannelReport
  subReport?: SubReport
}) {
  const { t, i18n } = useTranslation()
  const { resolvedTheme } = useTheme()
  const canvas = useRef<HTMLCanvasElement>(null)
  const host = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState(false)
  const [pageIndex, setPageIndex] = useState(0)
  const groupFilter = statusImageGroupFilter(props.config, 'channel')
  const modelFilter = props.config.channel_model_filter
  const pages = useMemo(
    () => channelReportPages(props.report, groupFilter, modelFilter),
    [props.report, groupFilter, modelFilter]
  )
  const currentPage = Math.min(pageIndex, pages.length - 1)

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
        // Canvas-only faces may not be requested by document.fonts.ready.
        const theme = getComputedStyle(host.current)
        const sans =
          theme.getPropertyValue('--font-sans').trim() || 'sans-serif'
        await Promise.all([
          document.fonts.load(`600 48px ${sans}`),
          document.fonts.load(`600 26px ${sans}`),
        ])
        if (cancelled || !canvas.current || !host.current) return
        renderStatusImage(canvas.current, {
          config: props.config,
          kind: props.kind,
          sample: props.sample,
          report: pages[currentPage],
          subReport: props.subReport,
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
    props.subReport,
    pages,
    currentPage,
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
      link.download = `${props.kind}-status${props.sample ? '-sample' : ''}-${currentPage + 1}.png`
      link.click()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    }, 'image/png')
  }

  return (
    <div className='flex min-w-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center justify-end gap-2'>
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
      {props.kind === 'channel' && pages.length > 1 && (
        <div className='flex items-center gap-3'>
          <Button
            type='button'
            variant='outline'
            size='icon'
            title={t('Previous page')}
            aria-label={t('Previous page')}
            disabled={currentPage === 0}
            onClick={() => setPageIndex(currentPage - 1)}
          >
            <ChevronLeft aria-hidden='true' />
          </Button>
          <span className='text-sm tabular-nums' aria-live='polite'>
            {currentPage + 1} / {pages.length}
          </span>
          <Button
            type='button'
            variant='outline'
            size='icon'
            title={t('Next page')}
            aria-label={t('Next page')}
            disabled={currentPage === pages.length - 1}
            onClick={() => setPageIndex(currentPage + 1)}
          >
            <ChevronRight aria-hidden='true' />
          </Button>
        </div>
      )}
      <div
        ref={host}
        data-theme-preset='anthropic'
        className='w-full min-w-0 overflow-auto bg-black'
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
          className={cn(
            'block h-auto w-full',
            props.expanded && 'min-w-[720px]'
          )}
        />
      </div>
    </div>
  )
}
