import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Save, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { getChannelStatusGroups } from '@/features/channel-status/api'

import { updateSystemOption } from '../api'
import { SettingsCard } from '../components/settings-card'
import { useSystemOptions } from '../hooks/use-system-options'
import type { ChannelStatusConfig, ChannelStatusGroupConfig } from '../types'

const defaultConfig: ChannelStatusConfig = {
  enabled: false,
  mode: 'user',
  groups: [],
}

function parseConfig(raw: string | undefined): ChannelStatusConfig {
  if (!raw) return defaultConfig
  try {
    const value = JSON.parse(raw) as Partial<ChannelStatusConfig>
    return {
      enabled: Boolean(value.enabled),
      mode:
        value.mode === 'api' || value.mode === 'user_then_api'
          ? value.mode
          : 'user',
      groups: Array.isArray(value.groups)
        ? value.groups.map((group) => ({
            group: group.group || '',
            enabled: Boolean(group.enabled),
            models: Array.isArray(group.models) ? group.models : [],
          }))
        : [],
    }
  } catch {
    return defaultConfig
  }
}

export function ChannelStatusSettings() {
  const queryClient = useQueryClient()
  const optionsQuery = useSystemOptions()
  const optionMap = useMemo(
    () =>
      new Map(
        optionsQuery.data?.data?.map((option) => [option.key, option.value])
      ),
    [optionsQuery.data]
  )
  const groupsQuery = useQuery({
    queryKey: ['channel-status-groups'],
    queryFn: getChannelStatusGroups,
  })
  const availableGroups = groupsQuery.data?.data?.items ?? []
  const [config, setConfig] = useState<ChannelStatusConfig>(defaultConfig)
  const [groupToAdd, setGroupToAdd] = useState('')

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(
    () => setConfig(parseConfig(optionMap.get('ChannelStatusConfig'))),
    [optionMap]
  )

  const saveMutation = useMutation({
    mutationFn: () =>
      updateSystemOption({
        key: 'ChannelStatusConfig',
        value: JSON.stringify(config),
      }),
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || '无法保存渠道状态设置')
        return
      }
      toast.success('渠道状态设置已保存')
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const updateGroup = (
    index: number,
    value: Partial<ChannelStatusGroupConfig>
  ) => {
    setConfig((previous) => ({
      ...previous,
      groups: previous.groups.map((group, itemIndex) =>
        itemIndex === index ? { ...group, ...value } : group
      ),
    }))
  }

  return (
    <SettingsCard
      title='渠道状态'
      description='选择渠道状态统计方式，不会改变原有渠道状态。'
    >
      <div className='space-y-5'>
        <div className='bg-muted/20 flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div>
            <Label htmlFor='channel-status-enabled'>开启渠道状态统计</Label>
            <p className='text-muted-foreground mt-1 text-xs'>
              可选择用户调用、API 调用，或先用户调用再 API 调用。
            </p>
          </div>
          <Switch
            id='channel-status-enabled'
            checked={config.enabled}
            onCheckedChange={(enabled) =>
              setConfig((previous) => ({ ...previous, enabled }))
            }
          />
        </div>
        <div className='space-y-2'>
          <Label>统计方式</Label>
          <Select
            value={config.mode}
            onValueChange={(mode) =>
              mode && setConfig((previous) => ({ ...previous, mode }))
            }
          >
            <SelectTrigger aria-label='选择渠道状态统计方式'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='user'>用户调用</SelectItem>
              <SelectItem value='api'>API 调用</SelectItem>
              <SelectItem value='user_then_api'>
                先用户调用，再 API 调用
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-3'>
          <div className='flex items-center justify-between'>
            <div>
              <Label>显示分组与统计模型</Label>
              <p className='text-muted-foreground mt-1 text-xs'>
                按已选择的统计方式展示渠道状态。
              </p>
            </div>
            <Select
              value={groupToAdd}
              onValueChange={(group) => {
                if (!group) return
                const groupOption = availableGroups.find(
                  (item) => item.group === group
                )
                if (!groupOption || groupOption.models.length === 0) {
                  toast.error('当前分组没有可用模型，无法添加该分组')
                  setGroupToAdd('')
                  return
                }
                setConfig((previous) => ({
                  ...previous,
                  groups: [
                    ...previous.groups,
                    { group, enabled: true, models: [groupOption.models[0]] },
                  ],
                }))
                setGroupToAdd('')
              }}
            >
              <SelectTrigger className='w-[180px]' aria-label='添加现有分组'>
                <Plus />
                <SelectValue placeholder='添加分组' />
              </SelectTrigger>
              <SelectContent>
                {groupsQuery.isLoading && (
                  <SelectItem value='__loading' disabled>
                    正在加载分组...
                  </SelectItem>
                )}
                {!groupsQuery.isLoading &&
                  availableGroups.filter(
                    (item) =>
                      !config.groups.some((group) => group.group === item.group)
                  ).length === 0 && (
                    <SelectItem value='__empty' disabled>
                      暂无可用分组
                    </SelectItem>
                  )}
                {availableGroups
                  .filter(
                    (item) =>
                      !config.groups.some((group) => group.group === item.group)
                  )
                  .map((item) => (
                    <SelectItem key={item.group} value={item.group}>
                      {item.group}
                      {item.models.length === 0 ? '（无模型）' : ''}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
          {groupsQuery.isError && (
            <p className='text-destructive text-xs'>
              读取现有分组失败，请刷新页面后重试。
            </p>
          )}
          {config.groups.map((group, index) => (
            <div
              key={`${group.group}-${group.models.join('-') || 'model'}`}
              className='grid gap-3 rounded-lg border p-3 md:grid-cols-[auto_1fr_2fr_auto] md:items-end'
            >
              <div className='flex items-center gap-2 pb-2'>
                <Switch
                  checked={group.enabled}
                  onCheckedChange={(enabled) => updateGroup(index, { enabled })}
                  aria-label='在渠道状态中显示分组'
                />
                <span className='text-sm'>显示</span>
              </div>
              <div className='space-y-2'>
                <Label>分组</Label>
                <div className='bg-muted/30 flex h-9 items-center rounded-md border px-3 text-sm'>
                  {group.group}
                </div>
              </div>
              <div className='space-y-2'>
                <Label>检测模型</Label>
                <MultiSelect
                  options={(
                    availableGroups.find((item) => item.group === group.group)
                      ?.models ?? []
                  ).map((model) => ({ label: model, value: model }))}
                  selected={group.models}
                  onChange={(models) => updateGroup(index, { models })}
                  placeholder='选择检测模型'
                  emptyText='当前分组没有模型'
                />
              </div>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                onClick={() =>
                  setConfig((previous) => ({
                    ...previous,
                    groups: previous.groups.filter(
                      (_, itemIndex) => itemIndex !== index
                    ),
                  }))
                }
                aria-label='移除分组'
              >
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
        <Button
          type='button'
          onClick={() => saveMutation.mutate()}
          disabled={saveMutation.isPending}
        >
          <Save />
          {saveMutation.isPending ? '保存中...' : '保存渠道状态设置'}
        </Button>
      </div>
    </SettingsCard>
  )
}
