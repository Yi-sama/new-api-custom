import { useMutation } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { SettingsCard } from '../components/settings-card'
import { getChannelHealth } from '../api'

type HealthKey = { index: number; key_preview: string; status: string; response_time_ms: number; error?: string }
type HealthChannel = { channel_id: number; name: string; type: number; base_url: string; status: string; keys: HealthKey[] }

function statusLabel(status: string) {
  if (status === 'healthy') return '正常'
  if (status === 'partial') return '部分可用'
  if (status === 'failed') return '失败'
  return status
}

export function ChannelHealthSettings() {
  const [items, setItems] = useState<HealthChannel[]>([])
  const [checkedAt, setCheckedAt] = useState<number | null>(null)
  const mutation = useMutation({
    mutationFn: getChannelHealth,
    onSuccess: (response) => {
      if (!response.success || !response.data) { toast.error(response.message || '检查失败'); return }
      setItems(response.data.items)
      setCheckedAt(response.data.checked_at)
      toast.success('渠道检查完成')
    },
    onError: (error: Error) => toast.error(error.message || '检查失败'),
  })
  return (
    <SettingsCard title='渠道健康监控' description='逐个检查渠道中的密钥；同一接口地址的渠道会分别显示，不代表彼此状态。'>
      <div className='mb-4 flex items-center gap-3'>
        <Button type='button' onClick={() => mutation.mutate()} disabled={mutation.isPending} aria-busy={mutation.isPending}><RefreshCw className={mutation.isPending ? 'animate-spin' : ''} />{mutation.isPending ? '检查中...' : '检查全部渠道'}</Button>
        {checkedAt && <span className='text-muted-foreground text-xs'>最近检查：{new Date(checkedAt * 1000).toLocaleString()}</span>}
      </div>
      {items.length === 0 ? <p className='text-muted-foreground text-sm'>点击“检查全部渠道”获取结果。</p> : <div className='space-y-3'>{items.map((channel) => <div key={channel.channel_id} className='rounded-md border p-3'><div className='flex flex-wrap items-center justify-between gap-2'><div><div className='font-medium'>{channel.name}</div><div className='text-muted-foreground text-xs'>{channel.base_url || '默认地址'}</div></div><span className='text-sm font-medium'>{statusLabel(channel.status)}</span></div><div className='mt-3 divide-y rounded border'>{channel.keys.map((key) => <div key={key.index} className='flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-sm'><span>密钥 {key.index + 1} · {key.key_preview}</span><span>{statusLabel(key.status)} · {key.response_time_ms} ms{key.error ? ` · ${key.error}` : ''}</span></div>)}</div></div>)}</div>}
    </SettingsCard>
  )
}
