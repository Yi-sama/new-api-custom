import { useQuery } from '@tanstack/react-query'
import {
  ArrowUpRight,
  CalendarClock,
  Check,
  Coins,
  Hourglass,
  RefreshCw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

import { getDailyTokenPool } from './api'

export function DailyTokenPoolView() {
  const { t, i18n } = useTranslation()
  const query = useQuery({
    queryKey: ['daily-token-pool'],
    queryFn: getDailyTokenPool,
    refetchInterval: 15_000,
    refetchOnWindowFocus: true,
    retry: 1,
  })
  const locale = toIntlLocale(i18n.language)
  const number = new Intl.NumberFormat(locale)
  const pool = query.data
  const remaining = pool?.enabled ? number.format(pool.remaining_tokens) : ''

  return (
    <section className='mx-auto flex w-full max-w-5xl flex-col gap-8 px-2 py-6 sm:px-6 sm:py-10'>
      <header className='flex items-start justify-between gap-4'>
        <div className='flex min-w-0 items-center gap-3'>
          <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-[8px]'>
            <Coins className='size-5' aria-hidden />
          </div>
          <div className='flex min-w-0 flex-col gap-1'>
            <h1 className='text-xl font-semibold break-words'>
              {t('Shared token pool')}
            </h1>
            {pool?.enabled && (
              <p className='text-muted-foreground text-sm break-words'>
                {pool.title === 'Daily shared token pool'
                  ? t('Daily shared token pool')
                  : pool.title}
              </p>
            )}
          </div>
        </div>
        <Button
          variant='outline'
          size='icon'
          className='shrink-0 rounded-[8px]'
          aria-label={t('Refresh')}
          title={t('Refresh')}
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          <RefreshCw
            className={cn(query.isFetching && 'motion-safe:animate-spin')}
          />
        </Button>
      </header>
      {query.isPending && (
        <div
          role='status'
          aria-label={t('Loading...')}
          className='flex flex-col gap-6'
        >
          <Skeleton className='h-40 w-full' />
          <Skeleton className='h-16 w-full' />
        </div>
      )}
      {query.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to load shared token pool')}</AlertTitle>
        </Alert>
      )}
      {!query.isError && pool && !pool.enabled && (
        <Empty role='status' className='min-h-64'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Coins />
            </EmptyMedia>
            <EmptyTitle>{t('Shared token pool is not available')}</EmptyTitle>
          </EmptyHeader>
        </Empty>
      )}
      {!query.isError && pool?.enabled && (
        <>
          <div className='grid min-w-0 gap-8 border-y py-8 md:grid-cols-[1.15fr_1fr] md:gap-12'>
            <div className='flex min-w-0 flex-col items-start justify-center gap-4'>
              <Badge variant={pool.exhausted ? 'destructive' : 'outline'}>
                {!pool.exhausted && (
                  <Check className='text-success size-3' aria-hidden />
                )}
                {pool.exhausted ? t('Daily pool exhausted') : t('Available')}
              </Badge>
              <dl className='flex min-w-0 flex-col gap-2'>
                <dt className='text-muted-foreground text-sm'>
                  {t('Available tokens')}
                </dt>
                <dd
                  className={cn(
                    'max-w-full font-semibold break-all tabular-nums',
                    remaining.length > 14
                      ? 'text-3xl'
                      : 'text-[40px] leading-tight'
                  )}
                >
                  {remaining}
                </dd>
              </dl>
              <span className='text-muted-foreground text-xs'>Token</span>
            </div>
            <dl className='flex min-w-0 flex-col justify-center divide-y'>
              {[
                {
                  label: 'Daily token allowance',
                  value: pool.daily_limit,
                  icon: Coins,
                  color: 'text-muted-foreground',
                },
                {
                  label: 'Tokens used today',
                  value: pool.used_tokens,
                  icon: ArrowUpRight,
                  color: 'text-info',
                },
                {
                  label: 'Tokens reserved',
                  value: pool.reserved_tokens,
                  icon: Hourglass,
                  color: 'text-warning',
                },
              ].map((item) => (
                <div
                  key={item.label}
                  className='flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-2 py-4 first:pt-0 last:pb-0'
                >
                  <dt className='text-muted-foreground flex min-w-0 items-center gap-2 text-sm'>
                    <item.icon
                      className={cn('size-4 shrink-0', item.color)}
                      aria-hidden
                    />
                    {t(item.label)}
                  </dt>
                  <dd className='text-lg font-medium break-all tabular-nums'>
                    {number.format(item.value)}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
          <div className='flex flex-col gap-3'>
            <div className='flex flex-wrap justify-between gap-2 text-sm'>
              <span className='text-muted-foreground'>
                {t('Remaining allowance')}
              </span>
              <span className='font-medium tabular-nums'>
                {new Intl.NumberFormat(locale, {
                  style: 'percent',
                  maximumFractionDigits: 2,
                }).format(pool.remaining_tokens / pool.daily_limit)}
              </span>
            </div>
            <Progress
              className='[&_[data-slot=progress-indicator]]:bg-success [&_[data-slot=progress-track]]:h-2'
              aria-label={t('Available tokens')}
              value={pool.remaining_tokens}
              max={pool.daily_limit}
            />
          </div>
          <div className='bg-muted/40 flex flex-wrap items-center justify-between gap-4 rounded-[8px] px-4 py-4 sm:px-5'>
            <div className='flex min-w-0 items-start gap-3'>
              <CalendarClock
                className='text-muted-foreground mt-0.5 size-5 shrink-0'
                aria-hidden
              />
              <div className='flex min-w-0 flex-col gap-1'>
                <span className='text-muted-foreground text-xs'>
                  {t('Next daily reset')}
                </span>
                <time
                  className='text-sm font-medium break-words'
                  dateTime={new Date(pool.reset_at * 1000).toISOString()}
                >
                  {new Intl.DateTimeFormat(locale, {
                    timeZone: pool.reset_timezone,
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  }).format(pool.reset_at * 1000)}
                </time>
              </div>
            </div>
            <Badge variant='outline'>{pool.reset_timezone}</Badge>
          </div>
          <Separator />
          <p className='text-muted-foreground text-xs tabular-nums'>
            {t('Pool updated at', {
              time: new Intl.DateTimeFormat(locale, {
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
              }).format(query.dataUpdatedAt),
            })}
          </p>
        </>
      )}
    </section>
  )
}

export function DailyTokenPoolPage() {
  return (
    <PublicLayout>
      <DailyTokenPoolView />
    </PublicLayout>
  )
}
