import type { TFunction } from 'i18next'

import {
  statusImageGroupFilter,
  type ReportKind,
  type StatusImageConfig,
} from './config'
import {
  channelReportComparisons,
  channelReportSections,
  filterSubReport,
  sampleSubReport,
  type ChannelReport,
  type ReportGroup,
  type SubReport,
} from './data'

type RenderOptions = {
  config: StatusImageConfig
  kind: ReportKind
  sample: boolean
  report: ChannelReport
  subReport?: SubReport
  theme: CSSStyleDeclaration
  logo: HTMLImageElement
  t: TFunction
}

const reportColors: Record<string, string> = {
  background: '#000000',
  foreground: '#f0f1f2',
  popover: '#121516',
  border: '#292e31',
  'muted-foreground': '#a6aeb2',
  success: '#78c9a5',
  warning: '#e3bc72',
  destructive: '#ec9298',
}

export function renderStatusImage(
  canvas: HTMLCanvasElement,
  options: RenderOptions
) {
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Canvas unavailable')
  const ctx = context
  const { config, t } = options
  const color = (name: string) =>
    reportColors[name] ?? options.theme.getPropertyValue(`--${name}`).trim()
  const sans =
    options.theme.getPropertyValue('--font-sans').trim() || 'sans-serif'
  const width = 1200
  const margin = 56
  const contentWidth = width - margin * 2
  const groupFilter = statusImageGroupFilter(config, options.kind)
  const sections = channelReportSections(
    options.report,
    groupFilter,
    config.channel_model_filter
  )
  let paint = true
  if (options.kind === 'sub' && !options.sample && !options.subReport) {
    throw new Error('Missing live Sub status')
  }
  const sub = filterSubReport(
    options.sample ? sampleSubReport : (options.subReport ?? sampleSubReport),
    groupFilter
  )
  const statusColor = (status: string) => {
    if (status === 'normal') return color('success')
    if (status === 'degraded' || status === 'cooling') return color('warning')
    if (status === 'failed') return color('destructive')
    return color('muted-foreground')
  }
  const statusText = (status: string) => {
    if (status === 'normal') return t('Available')
    if (status === 'degraded') return t('Degraded')
    if (status === 'cooling') return t('Cooling down')
    if (status === 'failed') return t('Unavailable')
    return t(options.kind === 'sub' ? 'Unknown' : 'No samples')
  }

  function text(
    value: string,
    x: number,
    y: number,
    maxWidth: number,
    size = 22,
    ink = color('foreground'),
    bold = false
  ) {
    ctx.font = `${bold ? '600 ' : ''}${size}px ${sans}`
    ctx.fillStyle = ink
    ctx.textBaseline = 'top'
    let cursor = y
    for (const paragraph of value.split(/\r?\n/)) {
      let line = ''
      for (const char of paragraph) {
        if (line && ctx.measureText(line + char).width > maxWidth) {
          if (paint) ctx.fillText(line, x, cursor)
          cursor += size * 1.35
          line = ''
        }
        line += char
      }
      if (paint) ctx.fillText(line, x, cursor)
      cursor += size * 1.35
    }
    return cursor
  }

  function rule(y: number) {
    ctx.fillStyle = color('border')
    ctx.fillRect(margin, y, contentWidth, 1)
  }

  function statusMark(status: string, x: number, y: number, maxWidth: number) {
    if (paint) {
      ctx.fillStyle = statusColor(status)
      ctx.beginPath()
      ctx.arc(x + 4, y + 11, 4, 0, Math.PI * 2)
      ctx.fill()
    }
    return text(
      statusText(status),
      x + 18,
      y,
      maxWidth - 18,
      17,
      color('muted-foreground')
    )
  }

  function groupCard(
    group: ReportGroup,
    x: number,
    top: number,
    cardWidth: number
  ) {
    const left = x + 28
    const inner = cardWidth - 56
    ctx.font = `17px ${sans}`
    const statusWidth = Math.min(
      inner * 0.45,
      ctx.measureText(statusText(group.items[0]?.status ?? 'unknown')).width +
        20
    )
    let y =
      Math.max(
        text(
          group.name,
          left,
          top + 26,
          inner - statusWidth - 20,
          22,
          color('foreground'),
          true
        ),
        statusMark(
          group.items[0]?.status ?? 'unknown',
          left + inner - statusWidth,
          top + 30,
          statusWidth
        )
      ) + 18
    for (const [index, item] of group.items.entries()) {
      if (index > 0 && paint) {
        ctx.fillStyle = color('border')
        ctx.fillRect(left, y, inner, 1)
      }
      if (index > 0) {
        y = statusMark(item.status, left, y + 18, inner) + 12
      }
      if (config.show_success_rate) {
        const value =
          item.availability === null ? '--' : `${item.availability.toFixed(1)}%`
        ctx.font = `600 52px ${sans}`
        const valueWidth = ctx.measureText(value).width
        y =
          Math.max(
            text(value, left, y, inner, 52, color('foreground'), true),
            text(
              t('24h success rate'),
              left + valueWidth + 14,
              y + 32,
              inner - valueWidth - 14,
              17,
              color('muted-foreground')
            )
          ) + 8
      }
      if (config.show_latency) {
        const value =
          item.latency === null ? '--' : `${(item.latency / 1000).toFixed(2)} s`
        ctx.font = `600 22px ${sans}`
        const valueWidth = ctx.measureText(value).width
        const bottom = text(
          t('Latest request duration'),
          left,
          y,
          inner - valueWidth - 20,
          18,
          color('muted-foreground')
        )
        y =
          Math.max(
            bottom,
            text(
              value,
              left + inner - valueWidth,
              y,
              valueWidth + 1,
              22,
              color('foreground'),
              true
            )
          ) + 8
      }
      if (config.show_updated_at) {
        const stamp = item.checkedAt
          ? new Date(item.checkedAt * 1000).toLocaleString()
          : '--'
        ctx.font = `17px ${sans}`
        const stampWidth = Math.min(inner - 140, ctx.measureText(stamp).width)
        const bottom = text(
          t('Last sample'),
          left,
          y,
          124,
          17,
          color('muted-foreground')
        )
        y =
          Math.max(
            bottom,
            text(stamp, left + inner - stampWidth, y, stampWidth + 1, 17)
          ) + 10
      }
      if (config.show_history) {
        y =
          text(
            t('Recent 12 hours'),
            left,
            y,
            inner,
            16,
            color('muted-foreground')
          ) + 8
        const history = item.history.length
          ? item.history.slice(-12)
          : Array<string>(12).fill('unknown')
        const blockWidth = (inner - (history.length - 1) * 4) / history.length
        if (paint) {
          history.forEach((status, block) => {
            ctx.fillStyle =
              status === 'unknown' ? color('border') : statusColor(status)
            ctx.beginPath()
            ctx.roundRect(left + block * (blockWidth + 4), y, blockWidth, 18, 2)
            ctx.fill()
          })
        }
        y += 18
      }
      y += 20
    }
    return y + 8
  }

  function subStatusBar(
    counts: number[],
    total: number,
    x: number,
    y: number,
    barWidth: number
  ) {
    if (!paint) return
    ctx.fillStyle = color('border')
    ctx.fillRect(x, y, barWidth, 8)
    let cursor = x
    for (const [index, count] of counts.entries()) {
      if (!count || !total) continue
      const segment = (count / total) * barWidth
      ctx.fillStyle = statusColor(
        ['normal', 'cooling', 'failed', 'unknown'][index]
      )
      ctx.fillRect(cursor, y, segment, 8)
      cursor += segment
    }
  }

  function subHeading(label: string, y: number) {
    ctx.font = `24px ${sans}`
    const labelWidth = Math.min(ctx.measureText(label).width, contentWidth - 24)
    const bottom = text(
      label,
      margin,
      y,
      contentWidth,
      24,
      color('foreground'),
      true
    )
    ctx.fillStyle = color('border')
    ctx.fillRect(
      margin + labelWidth + 24,
      y + 17,
      contentWidth - labelWidth - 24,
      1
    )
    return bottom + 24
  }

  function subGroupCard(
    group: SubReport['groups'][number],
    x: number,
    top: number,
    cardWidth: number
  ) {
    const left = x + 28
    const inner = cardWidth - 56
    let status = 'unknown'
    if (group.total > 0 && group.ready === group.total) status = 'normal'
    else if (group.ready > 0) status = 'degraded'
    else if (group.cooling > 0) status = 'cooling'
    else if (group.error > 0) status = 'failed'
    ctx.font = `17px ${sans}`
    const statusWidth = Math.min(
      inner * 0.45,
      ctx.measureText(statusText(status)).width + 20
    )
    let y =
      Math.max(
        text(
          group.name,
          left,
          top + 26,
          inner - statusWidth - 20,
          22,
          color('foreground'),
          true
        ),
        statusMark(status, left + inner - statusWidth, top + 30, statusWidth)
      ) + 18
    const value = `${group.ready} / ${group.total}`
    ctx.font = `600 48px ${sans}`
    const valueWidth = ctx.measureText(value).width
    y =
      Math.max(
        text(value, left, y, inner, 48, color('foreground'), true),
        text(
          t('Schedulable'),
          left + valueWidth + 14,
          y + 26,
          inner - valueWidth - 14,
          17,
          color('muted-foreground')
        )
      ) + 12
    const unknown = Math.max(
      0,
      group.total - group.ready - group.cooling - group.error
    )
    subStatusBar(
      [group.ready, group.cooling, group.error, unknown],
      group.total,
      left,
      y,
      inner
    )
    y += 24
    const counts = [
      { status: 'cooling', count: group.cooling },
      { status: 'failed', count: group.error },
    ]
    if (unknown) counts.push({ status: 'unknown', count: unknown })
    const columnWidth = inner / counts.length
    y =
      Math.max(
        ...counts.map((item, index) =>
          text(
            `${statusText(item.status)} ${item.count}`,
            left + index * columnWidth,
            y,
            columnWidth - 12,
            18,
            color('muted-foreground')
          )
        )
      ) + 26
    return y
  }

  function layout() {
    ctx.fillStyle = color('background')
    ctx.fillRect(0, 0, width, canvas.height)
    ctx.drawImage(options.logo, margin, 48, 42, 42)
    let y =
      text(
        config.site_name,
        margin + 58,
        54,
        contentWidth - 58,
        24,
        color('foreground')
      ) + 26
    y =
      text(
        options.kind === 'channel' ? config.channel_title : config.sub_title,
        margin,
        y,
        contentWidth,
        48,
        color('foreground'),
        true
      ) + 10
    y =
      text(
        options.sample
          ? t('Sample data · Not live status')
          : t(
              options.kind === 'sub'
                ? 'Account status'
                : 'Channel statistics · Last 24 hours'
            ),
        margin,
        y,
        contentWidth,
        20,
        color('muted-foreground')
      ) + 26
    if (
      options.kind === 'sub' &&
      !options.sample &&
      config.show_updated_at &&
      sub.checked_at
    ) {
      y =
        text(
          new Date(sub.checked_at * 1000).toLocaleString(),
          margin,
          y - 14,
          contentWidth,
          18,
          color('muted-foreground')
        ) + 22
    }
    rule(y)
    y += 32

    if (options.kind === 'sub') {
      if (sub.status !== 'connected') {
        y =
          text(
            t('Failed to load Sub account status'),
            margin,
            y,
            contentWidth,
            28
          ) + 18
        if (sub.error) {
          y =
            text(
              sub.error,
              margin,
              y,
              contentWidth,
              20,
              color('muted-foreground')
            ) + 24
        }
        return footer(y)
      }
      if (sub.groups.length === 0) {
        y =
          text(t('No matching account data'), margin, y, contentWidth, 28) + 60
        return footer(y)
      }
      const ready = sub.accounts.filter(
        (account) => account.status === 'normal'
      ).length
      const cooling = sub.accounts.filter(
        (account) => account.status === 'cooling'
      ).length
      const failed = sub.accounts.filter(
        (account) => account.status === 'failed'
      ).length
      const unknown = sub.accounts.length - ready - cooling - failed
      const summary = [
        {
          label: t('Schedulable accounts'),
          value: `${ready} / ${sub.accounts.length}`,
          status: 'normal',
        },
        { label: t('Cooling down'), value: String(cooling), status: 'cooling' },
        { label: t('Unavailable'), value: String(failed), status: 'failed' },
      ]
      if (unknown) {
        summary.push({
          label: t('Unknown'),
          value: String(unknown),
          status: 'unknown',
        })
      }
      const primaryWidth = unknown ? 420 : 560
      const secondaryWidth =
        (contentWidth - primaryWidth) / (summary.length - 1)
      y =
        Math.max(
          ...summary.map((item, index) => {
            const x =
              margin +
              (index === 0 ? 0 : primaryWidth + (index - 1) * secondaryWidth)
            const columnWidth = index === 0 ? primaryWidth : secondaryWidth
            const bottom = text(
              item.label,
              x,
              y,
              columnWidth - 24,
              20,
              color('muted-foreground')
            )
            return text(
              item.value,
              x,
              bottom + 8,
              columnWidth - 24,
              index === 0 ? 64 : 48,
              index === 0 ? color('success') : color('foreground'),
              true
            )
          })
        ) + 18
      subStatusBar(
        [ready, cooling, failed, unknown],
        sub.accounts.length,
        margin,
        y,
        contentWidth
      )
      y = subHeading(t('Group availability'), y + 54)
      const gap = 24
      const cardWidth = (contentWidth - gap) / 2
      const orderedGroups = [...sub.groups].sort(
        (left, right) =>
          (left.total ? left.ready / left.total : 1) -
          (right.total ? right.ready / right.total : 1)
      )
      for (let index = 0; index < orderedGroups.length; index += 2) {
        const row = orderedGroups.slice(index, index + 2)
        paint = false
        const rowBottom = Math.max(
          ...row.map((group, column) =>
            subGroupCard(
              group,
              margin + column * (cardWidth + gap),
              y,
              cardWidth
            )
          )
        )
        paint = true
        row.forEach((group, column) => {
          const x = margin + column * (cardWidth + gap)
          ctx.fillStyle = color('popover')
          ctx.strokeStyle = color('border')
          ctx.lineWidth = 1
          ctx.beginPath()
          ctx.roundRect(x, y, cardWidth, rowBottom - y, 6)
          ctx.fill()
          ctx.stroke()
          subGroupCard(group, x, y, cardWidth)
        })
        y = rowBottom + gap
      }
      const legend = [
        { status: 'normal', label: t('Schedulable') },
        { status: 'cooling', label: t('Cooling down') },
        { status: 'failed', label: t('Unavailable') },
      ]
      if (unknown) legend.push({ status: 'unknown', label: t('Unknown') })
      const legendWidth = contentWidth / legend.length
      y =
        Math.max(
          ...legend.map((item, index) => {
            const x = margin + index * legendWidth
            if (paint) {
              ctx.fillStyle = statusColor(item.status)
              ctx.beginPath()
              ctx.arc(x + 4, y + 12, 4, 0, Math.PI * 2)
              ctx.fill()
            }
            return text(
              item.label,
              x + 18,
              y,
              legendWidth - 30,
              18,
              color('muted-foreground')
            )
          })
        ) + 12
    } else if (!options.report.enabled || sections.length === 0) {
      y =
        text(
          t(
            options.report.enabled
              ? 'No matching channel data'
              : 'Channel statistics disabled'
          ),
          margin,
          y + 12,
          contentWidth,
          28
        ) + 60
    } else {
      const gap = 24
      for (const section of sections) {
        const columns = config.channel_columns
        const cardWidth = (contentWidth - gap * (columns - 1)) / columns
        const provider =
          section.provider === 'Other' ? t('Other') : section.provider
        ctx.font = `26px ${sans}`
        const labelWidth = Math.min(
          ctx.measureText(provider).width,
          contentWidth
        )
        text(provider, margin, y, contentWidth, 26, color('foreground'), true)
        ctx.fillStyle = color('border')
        ctx.fillRect(
          margin + labelWidth + 24,
          y + 18,
          contentWidth - labelWidth - 24,
          1
        )
        y += 52
        for (const model of channelReportComparisons(section)) {
          y =
            Math.max(
              text(
                model.name,
                margin,
                y,
                contentWidth - 172,
                28,
                color('foreground'),
                true
              ),
              text(
                `${model.groups.length} ${t('Groups')}`,
                margin + contentWidth - 152,
                y + 4,
                152,
                18,
                color('muted-foreground')
              )
            ) + 20
          for (let index = 0; index < model.groups.length; index += columns) {
            const row = model.groups.slice(index, index + columns)
            paint = false
            const rowBottom = Math.max(
              ...row.map((group, column) =>
                groupCard(
                  group,
                  margin + column * (cardWidth + gap),
                  y,
                  cardWidth
                )
              )
            )
            paint = true
            row.forEach((group, column) => {
              const x = margin + column * (cardWidth + gap)
              ctx.fillStyle = color('popover')
              ctx.strokeStyle = color('border')
              ctx.lineWidth = 1
              ctx.beginPath()
              ctx.roundRect(x, y, cardWidth, rowBottom - y, 6)
              ctx.fill()
              ctx.stroke()
              groupCard(group, x, y, cardWidth)
            })
            y = rowBottom + gap
          }
          y += 16
        }
        y += 16
      }
    }
    return footer(y)
  }

  function footer(cursor: number) {
    if (!config.footer) return cursor + margin
    let y = cursor
    y += 20
    rule(y)
    y += 24
    if (config.footer) {
      y =
        text(
          config.footer,
          margin,
          y,
          contentWidth,
          20,
          color('muted-foreground')
        ) + 16
    }
    return y + margin
  }

  // Measure and draw with the same layout so wrapped text cannot clip the PNG.
  canvas.width = width
  canvas.height = 1
  const height = layout()
  canvas.height = Math.ceil(height)
  layout()
}
