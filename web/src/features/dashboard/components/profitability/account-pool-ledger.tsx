import { Activity, CircleDollarSign } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import type {
  ProfitabilityCallComparison,
  ProfitabilityGroup,
  ProfitabilityReport,
  ProfitabilityRevenueBasis,
} from './api'

type CallScope = 'all_calls' | 'customer_calls'

function formatUsd(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

function formatRmb(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

function formatRatio(value: number): string {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 3,
    maximumFractionDigits: 6,
  }).format(value)
}

function statusLabel(status: string, t: (key: string) => string): string {
  switch (status) {
    case 'empty_selection':
      return t('No statistical channels selected')
    case 'not_configured':
      return t('Sub2API is not configured')
    case 'sync_error':
      return t('Sub2API sync failed')
    case 'scope_mismatch':
      return t('Selected scope does not cover the full pool')
    case 'cash_scope':
      return t('Cash view has no pool ratio')
    case 'unverified_scope':
      return t('Totals available; attribution incomplete')
    case 'no_usage':
      return t('No Sub usage in this period')
    case 'unmatched':
      return t('Customer Sub usage cannot be determined')
    case 'missing_purchase':
      return t('Purchase cost not recorded')
    case 'missing_ratio':
      return t('Call-time ratio is missing')
    case 'ratio_override':
      return t('Uses a channel or user-specific ratio')
    case 'no_paid_usage':
      return t('No paid usage in this period')
    case 'unallocated_purchase':
      return t('Purchase cost could not be allocated')
    case 'no_activity':
      return t('No activity in this period')
    case 'complete':
      return t('Complete')
    default:
      return t('Data not ready')
  }
}

function groupValue(
  value: number | null | undefined,
  formatter: (value: number) => string,
  t: (key: string) => string
): string {
  return value === null || value === undefined ? t('Unknown') : formatter(value)
}

function GroupRatioTable(props: {
  groups: ProfitabilityGroup[]
  callScope: CallScope
  revenueBasis: ProfitabilityRevenueBasis
}) {
  const { t } = useTranslation()

  return (
    <div className='border-b py-4'>
      <div className='mb-3'>
        <h3 id='group-ratio-title' className='text-sm font-semibold'>
          {t('Group ratio table')}
        </h3>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Purchase costs are allocated by group Sub usage within each channel. All-call break-even assumes every call is paid; customers-only uses paid customer revenue. Gift-funded calls still consume Sub usage.'
          )}
        </p>
      </div>
      <div
        className='max-w-full overflow-x-auto'
        role='region'
        aria-label={t('Group ratio table')}
        tabIndex={0}
      >
        <table className='w-full min-w-[58rem] table-fixed text-sm'>
          <thead className='text-muted-foreground text-left text-xs'>
            <tr className='border-y'>
              <th className='w-48 px-3 py-2.5 font-medium'>{t('Group')}</th>
              <th className='px-3 py-2.5 text-right font-medium'>
                {t('Charge (USD)')}
              </th>
              <th className='px-3 py-2.5 text-right font-medium'>
                {t('Sub usage (USD)')}
              </th>
              <th className='px-3 py-2.5 text-right font-medium'>
                {t('Actual charge / Sub ratio')}
              </th>
              <th className='px-3 py-2.5 text-right font-medium'>
                {t('Estimated group break-even ratio')}
              </th>
              <th className='px-3 py-2.5 text-right font-medium'>
                {t('Current configured ratio')}
              </th>
              <th className='w-44 px-3 py-2.5 text-right font-medium'>
                {t('Status')}
              </th>
            </tr>
          </thead>
          <tbody className='divide-y'>
            {props.groups.length === 0 ? (
              <tr>
                <td
                  className='text-muted-foreground px-3 py-6 text-center text-sm'
                  colSpan={7}
                >
                  {t('No group usage in this period')}
                </td>
              </tr>
            ) : (
              props.groups.map((group) => {
                const comparison = group[props.callScope]
                const breakEvenStatus =
                  props.revenueBasis === 'cash'
                    ? 'cash_scope'
                    : comparison?.break_even_status
                return (
                  <tr key={group.name}>
                    <th
                      className='px-3 py-2.5 text-left align-top font-medium break-words whitespace-normal'
                      scope='row'
                    >
                      {group.name}
                    </th>
                    <td className='px-3 py-2.5 text-right align-top break-all tabular-nums'>
                      {groupValue(comparison?.charge_usd, formatUsd, t)}
                    </td>
                    <td className='px-3 py-2.5 text-right align-top break-all tabular-nums'>
                      {groupValue(comparison?.sub_usage_usd, formatUsd, t)}
                    </td>
                    <td className='px-3 py-2.5 text-right align-top break-all tabular-nums'>
                      {groupValue(comparison?.effective_ratio, formatRatio, t)}
                    </td>
                    <td className='px-3 py-2.5 text-right align-top break-all tabular-nums'>
                      {breakEvenStatus === 'complete'
                        ? groupValue(
                            comparison?.break_even_ratio,
                            formatRatio,
                            t
                          )
                        : t('Unknown')}
                    </td>
                    <td className='px-3 py-2.5 text-right align-top break-all tabular-nums'>
                      {groupValue(group.configured_ratio, formatRatio, t)}
                    </td>
                    <td className='px-3 py-2.5 text-right align-top text-xs'>
                      {breakEvenStatus
                        ? statusLabel(breakEvenStatus, t)
                        : t('Unknown')}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function HeadlineMetric(props: {
  label: string
  value: string
  detail: string
  icon: typeof Activity
  accent?: boolean
}) {
  const Icon = props.icon
  return (
    <section
      className='min-h-28 min-w-0 px-4 py-4 sm:px-5'
      aria-label={props.label}
    >
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='text-muted-foreground text-xs font-medium'>
            {props.label}
          </p>
          <p
            className={cn(
              'mt-2 break-all text-xl font-semibold tabular-nums',
              props.accent && 'text-sky-700 dark:text-sky-300'
            )}
          >
            {props.value}
          </p>
          <p className='text-muted-foreground mt-1 text-xs'>{props.detail}</p>
        </div>
        <Icon
          className={cn(
            'text-muted-foreground size-4 shrink-0',
            props.accent && 'text-sky-600 dark:text-sky-400'
          )}
          aria-hidden='true'
        />
      </div>
    </section>
  )
}

function comparisonValue(
  comparison: ProfitabilityCallComparison | undefined,
  field: 'charge_usd' | 'sub_usage_usd' | 'effective_ratio'
): string {
  if (!comparison) return '-'
  const value = comparison[field]
  if (value === null) return '-'
  return field === 'effective_ratio' ? formatRatio(value) : formatUsd(value)
}

export function AccountPoolLedger(props: {
  report: ProfitabilityReport
  revenueBasis: ProfitabilityRevenueBasis
}) {
  const { t } = useTranslation()
  const [callScope, setCallScope] = useState<CallScope>('all_calls')
  const comparison = props.report.pool?.[callScope]
  const comparisonStatus = comparison?.status ?? 'data_not_ready'
  const usageLabel =
    callScope === 'all_calls'
      ? t('Full pool Sub usage')
      : t('Customer Sub usage')
  const operatingRevenue =
    props.revenueBasis === 'cash'
      ? props.report.cash_recognized_rmb
      : props.report.call_charge_rmb
  const profit = props.report.pool?.profit_rmb ?? null
  let profitTone = 'text-foreground'
  if (profit !== null) {
    profitTone =
      profit >= 0
        ? 'text-emerald-700 dark:text-emerald-300'
        : 'text-rose-700 dark:text-rose-300'
  }
  const hasMissingPurchase =
    props.report.pool?.profit_status === 'missing_purchase' ||
    props.report.channels.some(
      (channel) =>
        !channel.purchase_recorded &&
        channel.matched_requests + channel.unmatched_requests > 0
    )
  const hasRecordedPurchase = props.report.account_purchase_count > 0
  let purchaseValue = formatRmb(props.report.account_cost_rmb)
  let purchaseDetail = t('{{count}} accounts recorded', {
    count: props.report.account_count,
  })
  if (hasMissingPurchase && !hasRecordedPurchase) {
    purchaseValue = '-'
    purchaseDetail = t('Purchase cost not recorded')
  } else if (hasMissingPurchase) {
    purchaseDetail = t('Recorded purchase costs only')
  }
  const adminSubUsageKnown =
    props.report.pool?.all_calls.sub_usage_usd !== null &&
    props.report.pool?.all_calls.sub_usage_usd !== undefined

  return (
    <div className='space-y-5 tracking-normal'>
      <section aria-label={t('Account pool comparison')}>
        <div className='flex flex-col gap-3 border-b pb-3 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-muted-foreground text-xs'>
            {statusLabel(comparisonStatus, t)}
          </p>
          <div
            className='flex w-full rounded-md border p-0.5 sm:w-auto'
            role='group'
            aria-label={t('Call scope')}
          >
            <Button
              className='min-w-0 flex-1 sm:flex-none'
              size='sm'
              variant={callScope === 'all_calls' ? 'secondary' : 'ghost'}
              aria-pressed={callScope === 'all_calls'}
              onClick={() => setCallScope('all_calls')}
            >
              {t('Include administrators')}
            </Button>
            <Button
              className='min-w-0 flex-1 sm:flex-none'
              size='sm'
              variant={callScope === 'customer_calls' ? 'secondary' : 'ghost'}
              aria-pressed={callScope === 'customer_calls'}
              onClick={() => setCallScope('customer_calls')}
            >
              {t('Customers only')}
            </Button>
          </div>
        </div>

        <div className='grid divide-y border-b sm:grid-cols-2 sm:divide-x sm:divide-y-0'>
          <HeadlineMetric
            label={t('Total charge (USD)')}
            value={comparisonValue(comparison, 'charge_usd')}
            detail={statusLabel(comparisonStatus, t)}
            icon={CircleDollarSign}
          />
          <HeadlineMetric
            label={usageLabel}
            value={comparisonValue(comparison, 'sub_usage_usd')}
            detail={statusLabel(comparisonStatus, t)}
            icon={Activity}
            accent
          />
        </div>

        <GroupRatioTable
          groups={props.report.groups}
          callScope={callScope}
          revenueBasis={props.revenueBasis}
        />

        <details className='border-b'>
          <summary className='cursor-pointer py-3 text-sm font-medium'>
            {t('Scope comparison')}
          </summary>
          <div className='overflow-x-auto border-t'>
            <table className='w-full min-w-[42rem] text-sm'>
              <thead className='text-muted-foreground text-left text-xs'>
                <tr className='border-b'>
                  <th className='px-4 py-2.5 font-medium'>{t('Call scope')}</th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Charge (USD)')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Sub usage (USD)')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Charge-to-usage ratio')}
                  </th>
                  <th className='px-4 py-2.5 text-right font-medium'>
                    {t('Status')}
                  </th>
                </tr>
              </thead>
              <tbody className='divide-y'>
                {(['all_calls', 'customer_calls'] as const).map((scope) => {
                  const row = props.report.pool?.[scope]
                  return (
                    <tr key={scope}>
                      <th className='px-4 py-2.5 text-left font-medium'>
                        {scope === 'all_calls'
                          ? t('All calls')
                          : t('Customer calls')}
                      </th>
                      <td className='px-4 py-2.5 text-right tabular-nums'>
                        {comparisonValue(row, 'charge_usd')}
                      </td>
                      <td className='px-4 py-2.5 text-right tabular-nums'>
                        {comparisonValue(row, 'sub_usage_usd')}
                      </td>
                      <td className='px-4 py-2.5 text-right tabular-nums'>
                        {comparisonValue(row, 'effective_ratio')}
                      </td>
                      <td className='px-4 py-2.5 text-right text-xs'>
                        {statusLabel(row?.status ?? 'data_not_ready', t)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          <dl className='grid gap-x-6 gap-y-3 border-t px-4 py-3 text-sm sm:grid-cols-3'>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Administrator charge (USD)')}
              </dt>
              <dd className='mt-1 font-medium tabular-nums'>
                {props.report.pool
                  ? formatUsd(props.report.pool.admin_charge_usd)
                  : '-'}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Matched administrator Sub usage (USD)')}
              </dt>
              <dd className='mt-1 font-medium tabular-nums'>
                {adminSubUsageKnown && props.report.pool
                  ? formatUsd(props.report.pool.matched_admin_sub_usage_usd)
                  : '-'}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Outside selected channels Sub usage (USD)')}
              </dt>
              <dd className='mt-1 font-medium tabular-nums'>
                {adminSubUsageKnown && props.report.pool
                  ? formatUsd(props.report.pool.outside_selected_sub_usage_usd)
                  : '-'}
              </dd>
            </div>
          </dl>
        </details>
      </section>

      <section aria-labelledby='operating-ledger-title'>
        <div className='flex flex-wrap items-end justify-between gap-3 border-b pb-3'>
          <div>
            <h3 id='operating-ledger-title' className='font-semibold'>
              {t('Operating ledger')}
            </h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              {statusLabel(
                props.report.pool?.profit_status ?? 'data_not_ready',
                t
              )}
            </p>
          </div>
          {props.revenueBasis === 'cash' && (
            <div className='text-right' aria-label={t('Cash received (RMB)')}>
              <p className='text-muted-foreground text-xs'>
                {t('Cash received (RMB)')}
              </p>
              <p className='mt-1 text-sm font-semibold break-all tabular-nums'>
                {formatRmb(props.report.cash_received_rmb)}
              </p>
            </div>
          )}
        </div>
        <div className='grid divide-y border-b sm:grid-cols-3 sm:divide-x sm:divide-y-0'>
          <section
            className='min-h-24 min-w-0 px-4 py-4'
            aria-label={t('Customer paid usage revenue (RMB)')}
          >
            <p className='text-muted-foreground text-xs font-medium'>
              {t('Customer paid usage revenue (RMB)')}
            </p>
            <p className='mt-2 text-xl font-semibold break-all tabular-nums'>
              {formatRmb(operatingRevenue)}
            </p>
          </section>
          <section
            className='min-h-24 min-w-0 px-4 py-4'
            aria-label={t('Account purchase cost (RMB)')}
          >
            <p className='text-muted-foreground text-xs font-medium'>
              {t('Account purchase cost (RMB)')}
            </p>
            <p className='mt-2 text-xl font-semibold break-all tabular-nums'>
              {purchaseValue}
            </p>
            <p className='text-muted-foreground mt-1 text-xs'>
              {purchaseDetail}
            </p>
          </section>
          <section
            className='min-h-24 min-w-0 px-4 py-4'
            aria-label={t('Estimated profit (RMB)')}
          >
            <p className='text-muted-foreground text-xs font-medium'>
              {t('Estimated profit (RMB)')}
            </p>
            <p
              className={cn(
                'mt-2 break-all text-xl font-semibold tabular-nums',
                profitTone
              )}
            >
              {profit === null ? '-' : formatRmb(profit)}
            </p>
          </section>
        </div>
      </section>
    </div>
  )
}
