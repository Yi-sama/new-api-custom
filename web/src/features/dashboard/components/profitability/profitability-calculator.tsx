/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  CircleDollarSign,
  HandCoins,
  Plus,
  RefreshCw,
  ShoppingBag,
  TrendingUp,
  WalletCards,
} from 'lucide-react'
import { type FormEvent, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

import {
  addProfitabilityEntry,
  getProfitabilitySummary,
  type ProfitabilityEntryRequest,
  type ProfitabilityReport,
  type ProfitabilityTimeRange,
} from './api'

const EMPTY_REPORT: ProfitabilityReport = {
  revenue_usd: 0,
  topup_revenue_usd: 0,
  manual_income_usd: 0,
  admin_adjustment_usd: 0,
  account_cost_rmb: 0,
  sub_usage_usd: 0,
  cost_rate_rmb_per_usd: 0,
  account_count: 0,
  account_purchase_count: 0,
  sub_usage_status: 'not_configured',
  channels: [],
  models: [],
  accounts: [],
  unmatched_sub_requests: 0,
  cash_received_rmb: 0,
  cash_recognized_rmb: 0,
  cash_profit_rmb: null,
  call_charge_rmb: 0,
  estimated_profit_rmb: null,
  groups: [],
  usage: {
    quota: 0,
    amount_usd: 0,
    customer_usd: 0,
    self_usd: 0,
    request_count: 0,
    token_count: 0,
    users: [],
    groups: [],
  },
}

function formatUsd(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value || 0)
}

function formatRmb(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value || 0)
}

function formatInteger(value: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

function getCurrentMonthValue(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

function getMonthRange(month: string): ProfitabilityTimeRange {
  if (!month) return { startTimestamp: 0, endTimestamp: 0 }
  const [year, monthNumber] = month.split('-').map(Number)
  const start = new Date(year, monthNumber - 1, 1)
  const end = new Date(year, monthNumber, 0, 23, 59, 59, 999)
  return {
    startTimestamp: Math.floor(start.getTime() / 1000),
    endTimestamp: Math.floor(end.getTime() / 1000),
  }
}

function normalizeReport(
  report: ProfitabilityReport | null | undefined
): ProfitabilityReport {
  return {
    ...EMPTY_REPORT,
    ...report,
    groups: report?.groups ?? [],
    channels: report?.channels ?? [],
    models: report?.models ?? [],
    accounts: report?.accounts ?? [],
    usage: {
      ...EMPTY_REPORT.usage,
      ...report?.usage,
      users: report?.usage?.users ?? [],
      groups: report?.usage?.groups ?? [],
    },
  }
}

function SummaryMetric(props: {
  label: string
  value: string
  detail: string
  icon: typeof TrendingUp
  tone: 'green' | 'red' | 'blue' | 'amber'
}) {
  const Icon = props.icon
  return (
    <section
      className={cn(
        'rounded-lg border p-4',
        props.tone === 'green' &&
          'border-emerald-200 bg-emerald-50/70 dark:border-emerald-900/60 dark:bg-emerald-950/20',
        props.tone === 'red' &&
          'border-rose-200 bg-rose-50/70 dark:border-rose-900/60 dark:bg-rose-950/20',
        props.tone === 'blue' &&
          'border-sky-200 bg-sky-50/70 dark:border-sky-900/60 dark:bg-sky-950/20',
        props.tone === 'amber' &&
          'border-amber-200 bg-amber-50/70 dark:border-amber-900/60 dark:bg-amber-950/20'
      )}
    >
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground text-xs font-medium'>
            {props.label}
          </div>
          <div className='mt-2 truncate text-2xl font-semibold tabular-nums'>
            {props.value}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {props.detail}
          </div>
        </div>
        <Icon className='text-muted-foreground size-5 shrink-0' aria-hidden='true' />
      </div>
    </section>
  )
}

function EntryForm(props: {
  title: string
  description: string
  amountLabel: string
  amountCurrency: 'rmb' | 'usd'
  noteLabel: string
  submitLabel: string
  icon: typeof ShoppingBag
  entryType: ProfitabilityEntryRequest['entry_type']
  requiresAccountCount?: boolean
  channels?: ProfitabilityReport['channels']
  onSubmitted: (request: ProfitabilityEntryRequest) => Promise<void>
}) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [accountCount, setAccountCount] = useState('')
  const [channelId, setChannelId] = useState('')
  const [note, setNote] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const Icon = props.icon

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isSubmitting) return
    const numericAmount = Number(amount)
    const count = Number(accountCount)
    if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
      toast.error(
        props.amountCurrency === 'rmb'
          ? t('Enter a positive RMB amount')
          : t('Enter a positive USD amount')
      )
      return
    }
    if (
      props.requiresAccountCount &&
      (!Number.isInteger(count) || count <= 0)
    ) {
      toast.error(t('Enter a positive account count'))
      return
    }
    if (props.channels && !props.channels.some((channel) => channel.id === Number(channelId))) {
      toast.error(t('Select a channel'))
      return
    }

    setIsSubmitting(true)
    try {
      await props.onSubmitted({
        entry_type: props.entryType,
        amount_usd: props.amountCurrency === 'usd' ? numericAmount : 0,
        amount_rmb: props.amountCurrency === 'rmb' ? numericAmount : undefined,
        account_count: props.requiresAccountCount ? count : undefined,
        channel_id: props.channels ? Number(channelId) : undefined,
        note: note.trim(),
      })
      setAmount('')
      setAccountCount('')
      setNote('')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <section className='bg-card rounded-lg border p-4'>
      <div className='flex items-start gap-3'>
        <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
          <Icon className='size-4' aria-hidden='true' />
        </div>
        <div className='min-w-0'>
          <h3 className='font-semibold'>{props.title}</h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {props.description}
          </p>
        </div>
      </div>
      <form className='mt-4 space-y-3' onSubmit={handleSubmit}>
        {props.channels && (
          <div className='space-y-1.5'>
            <Label htmlFor={`${props.entryType}-channel`}>{t('Channel')}</Label>
            <select
              id={`${props.entryType}-channel`}
              className='bg-background h-9 w-full rounded-md border px-3 text-sm'
              value={channelId}
              onChange={(event) => setChannelId(event.target.value)}
            >
              <option value=''>{t('Select a channel')}</option>
              {props.channels.map((channel) => (
                <option key={channel.id} value={channel.id}>{channel.name} (#{channel.id})</option>
              ))}
            </select>
          </div>
        )}
        <div
          className={cn(
            'grid gap-3',
            props.requiresAccountCount && 'sm:grid-cols-2'
          )}
        >
          {props.requiresAccountCount && (
            <div className='space-y-1.5'>
              <Label htmlFor={`${props.entryType}-count`}>
                {t('Number of accounts')}
              </Label>
              <Input
                id={`${props.entryType}-count`}
                type='number'
                min={1}
                step={1}
                inputMode='numeric'
                value={accountCount}
                onChange={(event) => setAccountCount(event.target.value)}
              />
            </div>
          )}
          <div className='space-y-1.5'>
            <Label htmlFor={`${props.entryType}-amount`}>
              {props.amountLabel}
            </Label>
            <Input
              id={`${props.entryType}-amount`}
              type='number'
              min={0}
              step='0.01'
              inputMode='decimal'
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor={`${props.entryType}-note`}>{props.noteLabel}</Label>
          <Input
            id={`${props.entryType}-note`}
            value={note}
            placeholder={t('Optional note')}
            onChange={(event) => setNote(event.target.value)}
          />
        </div>
        <Button type='submit' disabled={isSubmitting}>
          <Plus data-icon='inline-start' />
          {props.submitLabel}
        </Button>
      </form>
    </section>
  )
}

function subStatusLabel(status: string, t: (key: string) => string): string {
  if (status === 'connected') return t('Auto-synced from Sub2API')
  if (status === 'error') return t('Sub2API sync failed')
  return t('Sub2API is not configured')
}

export function ProfitabilityCalculator() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [period, setPeriod] = useState<'all' | 'month'>('month')
  const [revenueMode, setRevenueMode] = useState<'calls' | 'cash'>('calls')
  const [month, setMonth] = useState(getCurrentMonthValue)
  const range = useMemo(
    () =>
      period === 'all'
        ? { startTimestamp: 0, endTimestamp: 0 }
        : getMonthRange(month),
    [month, period]
  )
  const reportQuery = useQuery({
    queryKey: ['profitability-summary', range.startTimestamp, range.endTimestamp],
    queryFn: () => getProfitabilitySummary(range),
  })
  const entryMutation = useMutation({
    mutationFn: addProfitabilityEntry,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['profitability-summary'] })
      toast.success(t('Profitability entry added'))
    },
    onError: (error: unknown) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to add entry')
      )
    },
  })
  const report = normalizeReport(reportQuery.data)
  const subStatus = subStatusLabel(report.sub_usage_status, t)
  const channelNames = new Map(report.channels.map((channel) => [channel.id, channel.name]))
  let profitValue = '-'
  if (report.estimated_profit_rmb !== null) {
    profitValue = formatRmb(report.estimated_profit_rmb)
  }
  let resultValue = profitValue
  if (revenueMode === 'cash') {
    resultValue = report.cash_profit_rmb === null ? '-' : formatRmb(report.cash_profit_rmb)
  }

  const submitEntry = async (request: ProfitabilityEntryRequest) => {
    await entryMutation.mutateAsync(request)
  }

  return (
    <div className='space-y-4'>
      <section className='flex flex-col justify-between gap-3 sm:flex-row sm:items-end'>
        <div>
          <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
            <CircleDollarSign className='size-4' aria-hidden='true' />
            {t('Break-even dashboard')}
          </div>
          <h2 className='mt-1 text-xl font-semibold tracking-tight'>
            {t('Channel and model profitability')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>{subStatus}</p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <div className='flex rounded-md border p-0.5' aria-label={t('Revenue basis')}>
            <Button size='sm' variant={revenueMode === 'calls' ? 'secondary' : 'ghost'} aria-pressed={revenueMode === 'calls'} onClick={() => setRevenueMode('calls')}>{t('Call charges')}</Button>
            <Button size='sm' variant={revenueMode === 'cash' ? 'secondary' : 'ghost'} aria-pressed={revenueMode === 'cash'} onClick={() => setRevenueMode('cash')}>{t('Cash received')}</Button>
          </div>
          <select
            aria-label={t('Report period')}
            className='bg-background h-9 rounded-md border px-3 text-sm'
            value={period}
            onChange={(event) =>
              setPeriod(event.target.value as 'all' | 'month')
            }
          >
            <option value='all'>{t('Cumulative')}</option>
            <option value='month'>{t('By month')}</option>
          </select>
          {period === 'month' && (
            <Input
              aria-label={t('Report month')}
              className='h-9 w-36'
              type='month'
              value={month}
              onChange={(event) => setMonth(event.target.value)}
            />
          )}
          <Button
            variant='outline'
            size='icon'
            aria-label={t('Refresh')}
            onClick={() => reportQuery.refetch()}
            disabled={reportQuery.isFetching}
          >
            <RefreshCw
              className={cn('size-4', reportQuery.isFetching && 'animate-spin')}
              aria-hidden='true'
            />
          </Button>
        </div>
      </section>
      {reportQuery.isError && <p role='alert' className='text-destructive text-sm'>{t('Failed to load profitability report')}</p>}
      {report.sub_usage_error && <p role='status' className='text-amber-700 text-sm'>{report.sub_usage_error}</p>}

      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6'>
        <SummaryMetric
          label={t('Account purchase cost')}
          value={formatRmb(report.account_cost_rmb)}
          detail={t('{{count}} accounts recorded', {
            count: formatInteger(report.account_count),
          })}
          icon={ShoppingBag}
          tone='red'
        />
        <SummaryMetric
          label={t('Sub usage')}
          value={formatUsd(report.sub_usage_usd)}
          detail={t('Automatically read from Sub2API')}
          icon={Activity}
          tone='blue'
        />
        <SummaryMetric
          label={revenueMode === 'calls' ? t('Customer call charges (RMB)') : t('Cash received (RMB)')}
          value={formatRmb(revenueMode === 'calls' ? report.call_charge_rmb : report.cash_received_rmb)}
          detail={revenueMode === 'calls' ? t('Gift-funded calls excluded') : t('Top-ups and recorded private payments')}
          icon={HandCoins}
          tone='green'
        />
        <SummaryMetric
          label={revenueMode === 'cash' ? t('Paid calls recognized (RMB)') : t('Customer usage')}
          value={revenueMode === 'cash' ? formatRmb(report.cash_recognized_rmb) : formatUsd(report.usage.customer_usd)}
          detail={revenueMode === 'cash' ? t('Unspent wallet funds are not profit') : t('Ordinary user calls')}
          icon={TrendingUp}
          tone='blue'
        />
        <SummaryMetric
          label={t('Your usage')}
          value={formatUsd(report.usage.self_usd)}
          detail={t('Administrator usage')}
          icon={WalletCards}
          tone='amber'
        />
        <SummaryMetric
          label={t('Estimated profit (RMB)')}
          value={resultValue}
          detail={revenueMode === 'calls' ? t('Unmatched calls prevent a complete estimate') : t('Unspent wallet funds are not profit')}
          icon={TrendingUp}
          tone='green'
        />
      </div>

      {revenueMode === 'calls' && <section className='bg-card rounded-lg border'>
        <div className='flex items-center justify-between gap-3 border-b px-4 py-3'>
          <div>
            <h3 className='font-semibold'>{t('Channel profitability')}</h3>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('Purchase costs are assigned by channel; Sub accounts are detected from calls.')}
            </p>
          </div>
          <TrendingUp className='size-4 text-emerald-600' aria-hidden='true' />
        </div>
        <div className='overflow-x-auto'>
          {report.channels.length === 0 ? (
            <p className='text-muted-foreground px-4 py-8 text-center text-sm'>
              {t('No channels in this period')}
            </p>
          ) : (
            <table className='w-full min-w-[44rem] text-sm'>
              <thead className='text-muted-foreground text-left text-xs'>
                <tr className='border-b'>
                  <th className='px-4 py-2.5 font-medium'>{t('Channel')}</th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Purchase cost (RMB)')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Sub usage (USD)')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Customer call charges (RMB)')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Estimated profit (RMB)')}
                  </th>
                </tr>
              </thead>
              <tbody className='divide-y'>
                {report.channels.map((channel) => (
                  <tr key={channel.id}>
                    <td className='px-4 py-2.5 font-medium'>{channel.name} (#{channel.id})</td>
                    <td className='px-4 py-2.5 text-right tabular-nums'>
                      {formatRmb(channel.purchase_rmb)}
                    </td>
                    <td className='px-4 py-2.5 text-right tabular-nums'>
                      {formatUsd(channel.sub_usage_usd)}
                    </td>
                    <td className='px-4 py-2.5 text-right tabular-nums'>{formatRmb(channel.customer_charge_rmb)}</td>
                    <td className='px-4 py-2.5 text-right tabular-nums'>
                      {channel.estimated_profit_rmb === null
                        ? t('Unmatched calls prevent a complete estimate')
                        : formatRmb(channel.estimated_profit_rmb)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>}

      {revenueMode === 'calls' && report.models.length > 0 && (
        <section className='bg-card rounded-lg border p-4'>
          <h3 className='font-semibold'>{t('Model profitability')}</h3>
          <div className='mt-3 overflow-x-auto'>
            <table className='w-full min-w-[36rem] text-sm'>
              <thead><tr className='border-b text-left'><th className='py-2'>{t('Channel')}</th><th>{t('Model')}</th><th className='text-right'>{t('Customer call charges (RMB)')}</th><th className='text-right'>{t('Allocated cost (RMB)')}</th><th className='text-right'>{t('Estimated profit (RMB)')}</th></tr></thead>
              <tbody className='divide-y'>{report.models.map((model) => (
                <tr key={`${model.channel_id}:${model.name}`}>
                  <td className='py-2'>{channelNames.get(model.channel_id) ?? `#${model.channel_id}`}</td><td>{model.name}</td>
                  <td className='text-right'>{formatRmb(model.customer_charge_rmb)}</td>
                  <td className='text-right'>{formatRmb(model.allocated_cost_rmb)}</td>
                  <td className='text-right'>{model.estimated_profit_rmb === null ? '-' : formatRmb(model.estimated_profit_rmb)}</td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        </section>
      )}
      {revenueMode === 'calls' && report.groups.length > 0 && (
        <section className='bg-card rounded-lg border p-4'>
          <h3 className='font-semibold'>{t('Group pricing')}</h3>
          <div className='mt-3 overflow-x-auto'>
            <table className='w-full min-w-[34rem] text-sm'>
              <thead><tr className='border-b text-left'><th className='py-2'>{t('Group')}</th><th className='text-right'>{t('Set ratio')}</th><th className='text-right'>{t('Estimated break-even ratio')}</th><th className='text-right'>{t('Estimated profit (RMB)')}</th></tr></thead>
              <tbody className='divide-y'>{report.groups.map((group) => (
                <tr key={group.name}><td className='py-2'>{group.name}</td><td className='text-right'>{group.ratio.toFixed(3)}</td><td className='text-right'>{group.break_even_ratio == null ? '-' : group.break_even_ratio.toFixed(3)}</td><td className='text-right'>{group.estimated_profit_rmb == null ? '-' : formatRmb(group.estimated_profit_rmb)}</td></tr>
              ))}</tbody>
            </table>
          </div>
        </section>
      )}
      {revenueMode === 'calls' && report.accounts.length > 0 && (
        <section className='bg-card rounded-lg border p-4'>
          <h3 className='font-semibold'>{t('Detected Sub accounts')}</h3>
          <div className='mt-2 flex flex-wrap gap-3 text-sm'>
            {report.accounts.map((account) => (
              <span key={`${account.channel_id}:${account.account_id}`}>{channelNames.get(account.channel_id) ?? `#${account.channel_id}`} · #{account.account_id}: {formatUsd(account.sub_usage_usd)}</span>
            ))}
          </div>
        </section>
      )}
      {revenueMode === 'cash' && <p className='text-muted-foreground text-sm'>{t('Cash view is site-wide. Channel and model results use call charges because wallet top-ups do not name a channel.')}</p>}

      <div className='grid gap-4 xl:grid-cols-2'>
        <EntryForm
          title={t('Record account purchase')}
          description={t('Enter the amount you paid in RMB once per purchase.')}
          amountLabel={t('Total cost (RMB)')}
          amountCurrency='rmb'
          noteLabel={t('Purchase note')}
          submitLabel={t('Add account cost')}
          icon={ShoppingBag}
          entryType='account_purchase'
          requiresAccountCount
          channels={report.channels}
          onSubmitted={submitEntry}
        />
        <EntryForm
          title={t('Record private payment')}
          description={t('Use this for money received outside online recharge.')}
          amountLabel={t('Received amount (USD)')}
          amountCurrency='usd'
          noteLabel={t('From / note')}
          submitLabel={t('Add income')}
          icon={HandCoins}
          entryType='manual_income'
          onSubmitted={submitEntry}
        />
      </div>
    </div>
  )
}
