/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronDown,
  CircleAlert,
  CircleDollarSign,
  Globe2,
  HandCoins,
  Plus,
  RefreshCw,
  ShoppingBag,
} from 'lucide-react'
import { type FormEvent, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { AccountPoolLedger } from './account-pool-ledger'
import {
  addProfitabilityEntry,
  getProfitabilitySummary,
  type ProfitabilityAvailableChannel,
  type ProfitabilityEntryRequest,
  type ProfitabilityReport,
  type ProfitabilityTimeRange,
} from './api'

const CHANNEL_SELECTION_STORAGE_PREFIX = 'profitability:channel-selection:'

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
  available_channels: [],
  scope: 'selected',
  selected_channel_ids: [],
  profit_status: 'empty_selection',
  matched_requests: 0,
  unmatched_requests: 0,
  unmatched_sub_usage_usd: 0,
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

function normalizeChannelIds(value: unknown): number[] {
  if (!Array.isArray(value)) return []
  return [
    ...new Set(
      value.filter(
        (id): id is number =>
          typeof id === 'number' && Number.isInteger(id) && id > 0
      )
    ),
  ]
    .map(Number)
    .sort((left, right) => left - right)
}

function loadChannelSelection(userId: number): number[] {
  if (!userId) return []
  try {
    return normalizeChannelIds(
      JSON.parse(
        localStorage.getItem(`${CHANNEL_SELECTION_STORAGE_PREFIX}${userId}`) ??
          '[]'
      )
    )
  } catch {
    return []
  }
}

function saveChannelSelection(userId: number, channelIds: number[]) {
  if (!userId) return
  try {
    localStorage.setItem(
      `${CHANNEL_SELECTION_STORAGE_PREFIX}${userId}`,
      JSON.stringify(channelIds)
    )
  } catch {
    // The selection still works for this session when storage is unavailable.
  }
}

function normalizeReport(report: ProfitabilityReport): ProfitabilityReport {
  return {
    ...EMPTY_REPORT,
    ...report,
    available_channels: report.available_channels ?? [],
    selected_channel_ids: report.selected_channel_ids ?? [],
    groups: report.groups ?? [],
    channels: report.channels ?? [],
    models: report.models ?? [],
    accounts: report.accounts ?? [],
    usage: {
      ...EMPTY_REPORT.usage,
      ...report.usage,
      users: report.usage?.users ?? [],
      groups: report.usage?.groups ?? [],
    },
  }
}

function subStatusLabel(status: string, t: (key: string) => string): string {
  if (status === 'connected') return t('Auto-synced from Sub2API')
  if (status === 'error') return t('Sub2API sync failed')
  return t('Sub2API is not configured')
}

function ChannelScopeSelector(props: {
  channels: ProfitabilityAvailableChannel[]
  selectedIds: number[]
  disabled: boolean
  onChange: (channelIds: number[]) => void
}) {
  const { t } = useTranslation()
  const selected = useMemo(
    () => new Set(props.selectedIds),
    [props.selectedIds]
  )
  const allCurrentIds = useMemo(
    () => props.channels.map((channel) => channel.id).sort((a, b) => a - b),
    [props.channels]
  )

  const toggleChannel = (channelId: number) => {
    const next = new Set(selected)
    if (next.has(channelId)) next.delete(channelId)
    else next.add(channelId)
    props.onChange([...next].sort((left, right) => left - right))
  }

  let triggerLabel = t('Choose channels')
  if (props.disabled) {
    triggerLabel = t('All channels (cash)')
  } else if (props.selectedIds.length > 0) {
    triggerLabel = t('{{count}} channels selected', {
      count: props.selectedIds.length,
    })
  }

  return (
    <Popover>
      <PopoverTrigger
        disabled={props.disabled}
        render={
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='max-w-full gap-1.5'
            aria-label={t('Statistical channels')}
          />
        }
      >
        {props.disabled ? (
          <Globe2 className='size-4' aria-hidden='true' />
        ) : (
          <CircleDollarSign className='size-4' aria-hidden='true' />
        )}
        <span className='truncate'>{triggerLabel}</span>
        <ChevronDown className='size-3.5 shrink-0' aria-hidden='true' />
      </PopoverTrigger>
      <PopoverContent
        className='w-[min(24rem,calc(100vw-1.5rem))] p-0'
        align='end'
      >
        <PopoverHeader className='px-3 pt-3'>
          <PopoverTitle>{t('Statistical channels')}</PopoverTitle>
        </PopoverHeader>
        <Command>
          <CommandInput placeholder={t('Search channels')} />
          <CommandList>
            <CommandEmpty>{t('No channels found')}</CommandEmpty>
            <CommandGroup heading={t('Selection')}>
              <CommandItem
                value={t('Select all current channels')}
                onSelect={() => props.onChange(allCurrentIds)}
              >
                {t('Select all current channels')}
              </CommandItem>
              <CommandItem
                value={t('Clear channel selection')}
                onSelect={() => props.onChange([])}
              >
                {t('Clear channel selection')}
              </CommandItem>
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading={t('Channels')}>
              {props.channels.map((channel) => {
                const checked = selected.has(channel.id)
                return (
                  <CommandItem
                    key={channel.id}
                    value={`${channel.name} ${channel.id} ${channel.status === 1 ? t('Enabled') : t('Disabled')}`}
                    aria-checked={checked}
                    onSelect={() => toggleChannel(channel.id)}
                  >
                    <Checkbox
                      checked={checked}
                      tabIndex={-1}
                      aria-hidden='true'
                      className='pointer-events-none'
                    />
                    <span className='min-w-0 flex-1 truncate'>
                      {channel.name} (#{channel.id})
                    </span>
                    {channel.status !== 1 && (
                      <Badge variant='outline' className='rounded-sm'>
                        {t('Disabled')}
                      </Badge>
                    )}
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
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
  channels?: ProfitabilityAvailableChannel[]
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
    if (
      props.channels &&
      !props.channels.some((channel) => channel.id === Number(channelId))
    ) {
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
                <option key={channel.id} value={channel.id}>
                  {channel.name} (#{channel.id})
                </option>
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

function ProfitabilityCalculatorForUser({ userId }: { userId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [period, setPeriod] = useState<'all' | 'month'>('month')
  const [revenueMode, setRevenueMode] = useState<'calls' | 'cash'>('calls')
  const [month, setMonth] = useState(getCurrentMonthValue)
  const [selectedChannelIds, setSelectedChannelIds] = useState(() =>
    loadChannelSelection(userId)
  )
  const range = useMemo(
    () =>
      period === 'all'
        ? { startTimestamp: 0, endTimestamp: 0 }
        : getMonthRange(month),
    [month, period]
  )
  const channelIdsKey = selectedChannelIds.join(',')
  const reportQuery = useQuery({
    queryKey: [
      'profitability-summary',
      userId,
      revenueMode,
      channelIdsKey,
      period,
      range.startTimestamp,
      range.endTimestamp,
    ],
    queryFn: () =>
      getProfitabilitySummary(
        range,
        revenueMode,
        revenueMode === 'calls' ? selectedChannelIds : undefined
      ),
    enabled: userId > 0,
    placeholderData: (previousReport) => previousReport,
  })
  const entryMutation = useMutation({
    mutationFn: addProfitabilityEntry,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['profitability-summary'],
      })
      toast.success(t('Profitability entry added'))
    },
    onError: (error: unknown) => {
      toast.error(
        error instanceof Error ? error.message : t('Failed to add entry')
      )
    },
  })
  const report =
    reportQuery.data && !reportQuery.isPlaceholderData
      ? normalizeReport(reportQuery.data)
      : null
  const availableChannels = reportQuery.data?.available_channels ?? []
  const channelNames = new Map(
    (report?.channels ?? []).map((channel) => [channel.id, channel.name])
  )
  const updateSelectedChannels = (channelIds: number[]) => {
    const normalized = normalizeChannelIds(channelIds)
    setSelectedChannelIds(normalized)
    saveChannelSelection(userId, normalized)
  }

  const submitEntry = async (request: ProfitabilityEntryRequest) => {
    await entryMutation.mutateAsync(request)
  }

  let reportContent = null
  if (reportQuery.isPending || reportQuery.isPlaceholderData) {
    reportContent = (
      <p
        role='status'
        className='text-muted-foreground py-8 text-center text-sm'
      >
        {t('Loading profitability report')}
      </p>
    )
  } else if (reportQuery.isError || !report) {
    reportContent = (
      <p role='alert' className='text-destructive py-8 text-center text-sm'>
        {t('Failed to load profitability report')}
      </p>
    )
  } else if (
    revenueMode === 'calls' &&
    report.profit_status === 'empty_selection'
  ) {
    reportContent = (
      <section className='bg-muted/30 rounded-lg border border-dashed px-4 py-8 text-center'>
        <CircleDollarSign
          className='text-muted-foreground mx-auto size-6'
          aria-hidden='true'
        />
        <h3 className='mt-3 font-semibold'>
          {t('No statistical channels selected')}
        </h3>
        <p className='text-muted-foreground mx-auto mt-1 max-w-lg text-sm'>
          {t('Choose channels')}
        </p>
      </section>
    )
  } else {
    const isSubSynced = report.sub_usage_status === 'connected'
    const hasActiveChannelWithoutPurchase = report.channels.some(
      (channel) =>
        !channel.purchase_recorded &&
        (channel.customer_charge_rmb > 0 ||
          channel.sub_usage_usd > 0 ||
          channel.matched_requests > 0 ||
          channel.unmatched_requests > 0)
    )
    const hasMissingPurchase =
      report.pool?.profit_status === 'missing_purchase' ||
      hasActiveChannelWithoutPurchase

    reportContent = (
      <>
        {report.sub_usage_error && (
          <p role='status' className='text-sm text-amber-700'>
            {report.sub_usage_error}
          </p>
        )}
        <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
          <span>
            {report.scope === 'all'
              ? t('Scope: all channels')
              : t('Scope: {{count}} selected channels', {
                  count: report.selected_channel_ids.length,
                })}
          </span>
          <span>
            {t('{{matched}} matched, {{unmatched}} unmatched requests', {
              matched: formatInteger(report.matched_requests),
              unmatched: formatInteger(report.unmatched_requests),
            })}
          </span>
          <span>{t('Sub scope: entire instance')}</span>
        </div>
        {hasMissingPurchase && (
          <p
            role='status'
            className='text-sm text-amber-700 dark:text-amber-300'
          >
            {t('Some selected channels have no purchase record')}
          </p>
        )}
        <AccountPoolLedger report={report} revenueBasis={revenueMode} />

        {(report.unmatched_sub_requests > 0 ||
          report.unmatched_sub_usage_usd > 0) && (
          <section className='border-warning/50 bg-warning/5 rounded-lg border px-4 py-3'>
            <div className='flex items-start gap-2'>
              <CircleAlert
                className='text-warning mt-0.5 size-4 shrink-0'
                aria-hidden='true'
              />
              <div>
                <h3 className='text-sm font-semibold'>
                  {t('Unassigned Sub usage (global)')}
                </h3>
                <p className='text-muted-foreground mt-0.5 text-xs'>
                  {t(
                    '{{count}} Sub requests totaling {{amount}} are included in full pool usage, but customer attribution is unconfirmed.',
                    {
                      count: formatInteger(report.unmatched_sub_requests),
                      amount: formatUsd(report.unmatched_sub_usage_usd),
                    }
                  )}
                </p>
              </div>
            </div>
          </section>
        )}

        {revenueMode === 'calls' && (
          <details className='border-b'>
            <summary className='cursor-pointer py-3 text-sm font-medium'>
              {t('Channel reconciliation')}
            </summary>
            <div className='overflow-x-auto'>
              {report.channels.length === 0 ? (
                <p className='text-muted-foreground px-4 py-8 text-center text-sm'>
                  {t('No channels in this period')}
                </p>
              ) : (
                <table className='w-full min-w-[46rem] text-sm'>
                  <thead className='text-muted-foreground text-left text-xs'>
                    <tr className='border-b'>
                      <th className='px-4 py-2.5 font-medium'>
                        {t('Channel')}
                      </th>
                      <th className='px-4 py-2.5 text-right font-medium'>
                        {t('Purchase cost (RMB)')}
                      </th>
                      <th className='px-4 py-2.5 text-right font-medium'>
                        {t('Matched Sub usage (USD)')}
                      </th>
                      <th className='px-4 py-2.5 text-right font-medium'>
                        {t('Matched / unmatched')}
                      </th>
                      <th className='px-4 py-2.5 text-right font-medium'>
                        {t('Customer call charges (RMB)')}
                      </th>
                    </tr>
                  </thead>
                  <tbody className='divide-y'>
                    {report.channels.map((channel) => {
                      const status =
                        channel.profit_status ?? report.profit_status
                      const subKnown =
                        isSubSynced &&
                        status !== 'not_configured' &&
                        status !== 'sync_error'
                      return (
                        <tr key={channel.id}>
                          <td className='px-4 py-2.5 font-medium'>
                            <div className='flex items-center gap-2'>
                              <span>
                                {channel.name} (#{channel.id})
                              </span>
                              {channel.status !== 1 && (
                                <Badge variant='outline' className='rounded-sm'>
                                  {t('Disabled')}
                                </Badge>
                              )}
                            </div>
                          </td>
                          <td className='px-4 py-2.5 text-right tabular-nums'>
                            {channel.purchase_recorded
                              ? formatRmb(channel.purchase_rmb)
                              : '-'}
                          </td>
                          <td className='px-4 py-2.5 text-right tabular-nums'>
                            {subKnown ? formatUsd(channel.sub_usage_usd) : '-'}
                          </td>
                          <td className='px-4 py-2.5 text-right tabular-nums'>
                            {formatInteger(channel.matched_requests)} /{' '}
                            {formatInteger(channel.unmatched_requests)}
                          </td>
                          <td className='px-4 py-2.5 text-right tabular-nums'>
                            {formatRmb(channel.customer_charge_rmb)}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
          </details>
        )}

        {revenueMode === 'calls' && report.accounts.length > 0 && (
          <details className='border-b'>
            <summary className='cursor-pointer py-3 text-sm font-medium'>
              {t('Detected Sub accounts')}
            </summary>
            <div className='flex flex-wrap gap-3 pb-3 text-sm'>
              {report.accounts.map((account) => (
                <span key={`${account.channel_id}:${account.account_id}`}>
                  {channelNames.get(account.channel_id) ??
                    `#${account.channel_id}`}{' '}
                  · #{account.account_id}: {formatUsd(account.sub_usage_usd)}
                </span>
              ))}
            </div>
          </details>
        )}
      </>
    )
  }

  return (
    <div className='space-y-4 tracking-normal'>
      <section className='flex flex-col justify-between gap-3 lg:flex-row lg:items-end'>
        <div>
          <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
            <CircleDollarSign className='size-4' aria-hidden='true' />
            {t('Profitability')}
          </div>
          <h2 className='mt-1 text-xl font-semibold'>
            {t('Account pool ledger')}
          </h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            {report
              ? subStatusLabel(report.sub_usage_status, t)
              : t('Loading profitability report')}
          </p>
        </div>
        <div className='flex max-w-full flex-wrap items-center gap-2'>
          <ChannelScopeSelector
            channels={availableChannels}
            selectedIds={selectedChannelIds}
            disabled={revenueMode === 'cash'}
            onChange={updateSelectedChannels}
          />
          <div
            className='flex rounded-md border p-0.5'
            aria-label={t('Revenue basis')}
          >
            <Button
              size='sm'
              variant={revenueMode === 'calls' ? 'secondary' : 'ghost'}
              aria-pressed={revenueMode === 'calls'}
              onClick={() => setRevenueMode('calls')}
            >
              {t('Call charges')}
            </Button>
            <Button
              size='sm'
              variant={revenueMode === 'cash' ? 'secondary' : 'ghost'}
              aria-pressed={revenueMode === 'cash'}
              onClick={() => setRevenueMode('cash')}
            >
              {t('Cash received')}
            </Button>
          </div>
          <select
            aria-label={t('Report period')}
            className='bg-background h-9 max-w-full rounded-md border px-3 text-sm'
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
              className='h-9 w-36 max-w-full'
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

      {reportContent}

      {(report || availableChannels.length > 0) && (
        <div
          className={cn(
            'grid gap-4',
            revenueMode === 'cash' && 'xl:grid-cols-2'
          )}
        >
          <EntryForm
            title={t('Record account purchase')}
            description={t(
              'Enter the amount you paid in RMB once per purchase.'
            )}
            amountLabel={t('Total cost (RMB)')}
            amountCurrency='rmb'
            noteLabel={t('Purchase note')}
            submitLabel={t('Add account cost')}
            icon={ShoppingBag}
            entryType='account_purchase'
            requiresAccountCount
            channels={availableChannels.filter(
              (channel) => channel.status === 1
            )}
            onSubmitted={submitEntry}
          />
          {revenueMode === 'cash' && (
            <EntryForm
              title={t('Record private payment')}
              description={t(
                'Use this for money received outside online recharge.'
              )}
              amountLabel={t('Received amount (USD)')}
              amountCurrency='usd'
              noteLabel={t('From / note')}
              submitLabel={t('Add income')}
              icon={HandCoins}
              entryType='manual_income'
              onSubmitted={submitEntry}
            />
          )}
        </div>
      )}
    </div>
  )
}

export function ProfitabilityCalculator() {
  const userId = useAuthStore((state) => state.auth.user?.id ?? 0)
  return <ProfitabilityCalculatorForUser key={userId} userId={userId} />
}
