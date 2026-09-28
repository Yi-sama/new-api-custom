import { useQuery } from '@tanstack/react-query'
import { Activity, Check, ChevronDown, CircleAlert, CircleX, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { getLobeIcon } from '@/lib/lobe-icon'
import { getChannelStatusOverview, type ChannelStatusCard } from './api'

function statusIcon(status: ChannelStatusCard['status']) {
  if (status === 'normal') return <Check className='size-4' />
  if (status === 'degraded') return <CircleAlert className='size-4' />
  if (status === 'failed') return <CircleX className='size-4' />
  return <Activity className='size-4' />
}

function statusClass(status: ChannelStatusCard['status']) {
  if (status === 'normal') return 'border-emerald-500/30 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400'
  if (status === 'degraded') return 'border-amber-500/30 bg-amber-500/5 text-amber-600 dark:text-amber-400'
  if (status === 'failed') return 'border-rose-500/30 bg-rose-500/5 text-rose-600 dark:text-rose-400'
  return 'border-muted bg-muted/20 text-muted-foreground'
}

function statusLabel(status: ChannelStatusCard['status']) {
  return { normal: '正常', degraded: '性能下降', failed: '失败', unknown: '未知' }[status]
}

function blockClass(status: string) {
  if (status === 'normal') return 'bg-emerald-500'
  if (status === 'degraded') return 'bg-amber-500'
  if (status === 'failed') return 'bg-rose-500'
  return 'bg-muted'
}

function modelProvider(modelName: string) {
  const model = modelName.toLowerCase()
  if (model.includes('claude') || model.includes('anthropic')) return { icon: 'Claude.Color', name: 'Claude' }
  if (model.includes('gemini') || model.includes('gemma')) return { icon: 'Gemini.Color', name: 'Gemini' }
  if (model.includes('deepseek')) return { icon: 'DeepSeek.Color', name: 'DeepSeek' }
  if (model.includes('qwen') || model.includes('qwq')) return { icon: 'Qwen.Color', name: 'Qwen' }
  if (model.includes('grok') || model.includes('xai')) return { icon: 'Grok.Color', name: 'Grok' }
  return { icon: 'OpenAI.Color', name: 'OpenAI' }
}

function ChannelStatusCardView(props: { card: ChannelStatusCard; groupName: string }) {
  const card = props.card
  return (
    <Card className='overflow-hidden border-border/70 bg-card/80 shadow-sm'>
      <CardHeader className='space-y-2 pb-3'>
        <div className='flex items-start justify-between gap-2'>
          <div className='min-w-0'>
            <CardTitle className='truncate text-sm'>{props.groupName}</CardTitle>
          </div>
          <Badge variant='outline' className={statusClass(card.status)}>{statusIcon(card.status)}{statusLabel(card.status)}</Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-3 pt-0'>
        <div className='grid grid-cols-3 gap-2 text-xs'><div><div className='text-muted-foreground'>24 小时成功率</div><div className='font-semibold'>{card.status === 'unknown' ? '--' : `${card.availability_24h.toFixed(1)}%`}</div></div><div><div className='text-muted-foreground'>最近延迟</div><div className='font-semibold'>{card.status === 'unknown' ? '--' : `${card.latest_latency_ms} ms`}</div></div><div><div className='text-muted-foreground'>平均延迟</div><div className='font-semibold'>{card.status === 'unknown' ? '--' : `${Math.round(card.average_latency_ms)} ms`}</div></div></div>
        <div className='flex gap-1' aria-label='最近调用结果'>
          {card.blocks.map((block) => <span key={block.started_at} className={`h-2 flex-1 rounded-sm ${blockClass(block.status)}`} title={statusLabel(block.status as ChannelStatusCard['status'])} />)}
        </div>
      </CardContent>
    </Card>
  )
}

export function ChannelStatusPage() {
  const overview = useQuery({ queryKey: ['channel-status-overview'], queryFn: getChannelStatusOverview, refetchInterval: 30_000 })
  const [expandedModels, setExpandedModels] = useState<Record<string, boolean>>({})
  const data = overview.data?.data
  const models = useMemo(() => {
    const modelNames = new Set<string>()
    data?.groups?.forEach((group) => group.models?.forEach((model) => modelNames.add(model.model)))
    return [...modelNames]
  }, [data])
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>渠道状态</SectionPageLayout.Title>
      <SectionPageLayout.Actions><Button variant='outline' size='sm' onClick={() => void overview.refetch()} disabled={overview.isFetching}><RefreshCw className={overview.isFetching ? 'animate-spin' : ''} />刷新</Button></SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {!data?.enabled && <div className='rounded-xl border border-dashed p-10 text-center text-sm text-muted-foreground'>当前暂无可展示的渠道状态信息，请稍后再试。</div>}
        {data?.enabled && <div className='space-y-5'>
          {models.map((modelName) => {
            const isExpanded = expandedModels[modelName] ?? modelName === models[0]
            const provider = modelProvider(modelName)
            return (
              <section key={modelName} className='overflow-hidden rounded-xl border border-border bg-card/30'>
                <button
                  type='button'
                  className='flex w-full items-center justify-between gap-4 border-b border-border px-5 py-4 text-left transition-colors hover:bg-muted/40'
                  aria-expanded={isExpanded}
                  onClick={() => setExpandedModels((previous) => ({ ...previous, [modelName]: !isExpanded }))}
                >
                  <span className='flex min-w-0 items-center gap-3'>
                    <span className='flex size-7 shrink-0 items-center justify-center'>{getLobeIcon(provider.icon, 24)}</span>
                    <span className='text-base font-semibold'>{modelName}</span>
                    <span className='text-muted-foreground text-xs'>{provider.name} · 真实调用统计</span>
                  </span>
                  <ChevronDown className={`text-muted-foreground size-5 transition-transform ${isExpanded ? 'rotate-180' : ''}`} aria-hidden='true' />
                </button>
                {isExpanded && <div className='p-4'>
                  <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
                    {data.groups?.flatMap((group) => group.models?.find((item) => item.model === modelName)?.cards?.map((card) => ({ card, groupName: group.group })) ?? []).map((item) => <ChannelStatusCardView key={`${item.groupName}-${item.card.channel_id}`} card={item.card} groupName={item.groupName} />)}
                  </div>
                </div>}
              </section>
            )
          })}
        </div>}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
