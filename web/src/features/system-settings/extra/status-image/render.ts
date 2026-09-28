import type { TFunction } from 'i18next'

import type { ReportKind, StatusImageConfig } from './config'
import { sampleSubAccounts, sampleSubGroups, type ChannelReport } from './data'

type RenderOptions = {
  config: StatusImageConfig
  kind: ReportKind
  sample: boolean
  report: ChannelReport
  theme: CSSStyleDeclaration
  logo: HTMLImageElement
  t: TFunction
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
    options.theme.getPropertyValue(`--${name}`).trim()
  const font = options.theme.getPropertyValue('--font-serif').trim() || 'serif'
  const sans =
    options.theme.getPropertyValue('--font-sans').trim() || 'sans-serif'
  const width = 1200
  const margin = 56
  const contentWidth = width - margin * 2
  const filtered = new Set(
    config.group_filter
      .split(/[,\n]/)
      .map((value) => value.trim())
      .filter(Boolean)
  )
  const groups = options.report.groups.filter(
    (group) => filtered.size === 0 || filtered.has(group.name)
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
    return t('No samples')
  }

  function text(
    value: string,
    x: number,
    y: number,
    maxWidth: number,
    size = 22,
    ink = color('foreground'),
    serif = false
  ) {
    ctx.font = `${size}px ${serif ? font : sans}`
    ctx.fillStyle = ink
    ctx.textBaseline = 'top'
    let cursor = y
    for (const paragraph of value.split(/\r?\n/)) {
      let line = ''
      for (const char of paragraph) {
        if (line && ctx.measureText(line + char).width > maxWidth) {
          ctx.fillText(line, x, cursor)
          cursor += size * 1.5
          line = ''
        }
        line += char
      }
      ctx.fillText(line, x, cursor)
      cursor += size * 1.5
    }
    return cursor
  }

  function rule(y: number) {
    ctx.fillStyle = color('border')
    ctx.fillRect(margin, y, contentWidth, 1)
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
        26,
        color('foreground'),
        true
      ) + 26
    y =
      text(
        options.kind === 'channel' ? config.channel_title : config.sub_title,
        margin,
        y,
        contentWidth,
        44,
        color('foreground'),
        true
      ) + 10
    y =
      text(
        options.sample
          ? t('Sample data · Not live status')
          : t('Channel statistics · Last 24 hours'),
        margin,
        y,
        contentWidth,
        20,
        color('muted-foreground')
      ) + 26
    rule(y)
    y += 32

    if (options.kind === 'sub') {
      y = text(
        t('Schedulable accounts'),
        margin,
        y,
        contentWidth,
        22,
        color('muted-foreground')
      )
      y =
        text(
          '10 / 12',
          margin,
          y + 8,
          contentWidth,
          48,
          color('foreground'),
          true
        ) + 20
      const columns = [margin, 440, 635, 830]
      const headers = [
        t('Group'),
        t('Schedulable'),
        t('Cooling down'),
        t('Unavailable'),
      ]
      for (let i = 0; i < headers.length; i++) {
        text(
          headers[i],
          columns[i],
          y,
          i === 0 ? 360 : 180,
          20,
          color('muted-foreground')
        )
      }
      y += 66
      for (const group of sampleSubGroups) {
        rule(y - 14)
        text(group.name, columns[0], y, 360, 28, color('foreground'), true)
        text(`${group.ready} / ${group.total}`, columns[1], y, 180, 26)
        text(String(group.cooling), columns[2], y, 180, 26)
        text(String(group.error), columns[3], y, 180, 26)
        y += 80
      }
      if (config.show_sub_details) {
        y += 28
        y =
          text(
            t('Account excerpts'),
            margin,
            y,
            contentWidth,
            28,
            color('foreground'),
            true
          ) + 18
        for (const account of sampleSubAccounts) {
          rule(y - 10)
          text(account.name, margin, y + 8, 440, 23)
          text(account.group, 530, y + 8, 200, 22, color('muted-foreground'))
          ctx.fillStyle = statusColor(account.status)
          ctx.fillRect(840, y + 16, 10, 10)
          text(
            statusText(account.status),
            862,
            y + 8,
            278,
            22,
            color('foreground')
          )
          y += 70
        }
      }
    } else if (!options.report.enabled || groups.length === 0) {
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
      const cardWidth =
        (contentWidth - gap * (config.channel_columns - 1)) /
        config.channel_columns
      let count = 0
      for (const group of groups) {
        if (count >= 60) break
        y =
          text(
            group.name,
            margin,
            y,
            contentWidth,
            28,
            color('foreground'),
            true
          ) + 18
        const items = group.items.slice(0, 60 - count)
        for (
          let index = 0;
          index < items.length;
          index += config.channel_columns
        ) {
          let rowBottom = y
          for (
            let column = 0;
            column < config.channel_columns && index + column < items.length;
            column++
          ) {
            const item = items[index + column]
            const x = margin + column * (cardWidth + gap)
            const inner = cardWidth - 40
            let bottom =
              text(
                item.name,
                x + 20,
                y + 22,
                inner,
                27,
                color('foreground'),
                true
              ) + 12
            ctx.fillStyle = statusColor(item.status)
            ctx.fillRect(x + 20, bottom + 8, 10, 10)
            bottom =
              text(
                statusText(item.status),
                x + 42,
                bottom,
                inner - 22,
                20,
                color('foreground')
              ) + 16
            if (config.show_success_rate) {
              const value =
                item.availability === null
                  ? '-'
                  : `${item.availability.toFixed(1)}%`
              bottom =
                text(
                  `${t('24h success rate')}   ${value}`,
                  x + 20,
                  bottom,
                  inner,
                  20
                ) + 6
            }
            if (config.show_latency) {
              const value =
                item.latency === null ? '-' : `${Math.round(item.latency)} ms`
              bottom =
                text(
                  `${t('Latest request duration')}   ${value}`,
                  x + 20,
                  bottom,
                  inner,
                  20
                ) + 6
            }
            if (config.show_history) {
              const history = item.history.length
                ? item.history.slice(-12)
                : Array<string>(12).fill('unknown')
              const blockWidth =
                (inner - (history.length - 1) * 5) / history.length
              history.forEach((status, block) => {
                ctx.fillStyle = statusColor(status)
                ctx.fillRect(
                  x + 20 + block * (blockWidth + 5),
                  bottom + 8,
                  blockWidth,
                  16
                )
              })
              bottom += 38
              bottom =
                text(
                  t('Recent 12 hours'),
                  x + 20,
                  bottom,
                  inner,
                  17,
                  color('muted-foreground')
                ) + 8
            }
            if (config.show_updated_at) {
              const stamp = item.checkedAt
                ? new Date(item.checkedAt * 1000).toLocaleString()
                : '-'
              bottom =
                text(
                  `${t('Last sample')}   ${stamp}`,
                  x + 20,
                  bottom,
                  inner,
                  17,
                  color('muted-foreground')
                ) + 10
            }
            bottom += 16
            ctx.strokeStyle = color('border')
            ctx.lineWidth = 1
            ctx.strokeRect(x, y, cardWidth, bottom - y)
            rowBottom = Math.max(rowBottom, bottom)
          }
          y = rowBottom + gap
        }
        count += items.length
        y += 12
      }
      if (groups.reduce((total, group) => total + group.items.length, 0) > 60) {
        y = text(t('First 60 items shown'), margin, y, contentWidth, 20) + 20
      }
    }
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
    y = text(
      'New API · QuantumNous',
      margin,
      y,
      contentWidth,
      18,
      color('muted-foreground')
    )
    return y + margin
  }

  // Measure and draw with the same layout so wrapped text cannot clip the PNG.
  canvas.width = width
  canvas.height = 1
  const height = layout()
  canvas.height = Math.ceil(height)
  layout()
}
