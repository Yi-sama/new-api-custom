/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDownToLine, Plus, RefreshCw, Save, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { getChannels } from '@/features/channels/api'

import { previewNikoGroupSync, saveNikoGroupSync } from '../api'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsCard } from '../components/settings-card'
import { useSystemOptions } from '../hooks/use-system-options'
import type {
  NikoGroupMapping,
  NikoGroupPreview,
  NikoGroupSyncConfig,
} from '../types'

const defaultMappings: NikoGroupMapping[] = []

const defaultConfig: NikoGroupSyncConfig = {
  enabled: false,
  source_url: 'https://us.nikoapi.xyz/api/pricing',
  timeout_seconds: 10,
  ratio_adjustment: 0,
  base_group_ratio: {},
  mappings: defaultMappings,
}

function readNikoGroupSyncConfig(
  raw: string | undefined,
  availableGroups: string[] = []
): NikoGroupSyncConfig {
  if (!raw) {
    const mappings = availableGroups.map((group) => ({
      id: `niko-${group}`,
      enabled: true,
      local_group: group,
      remote_group: '',
      ratio_mode: 'remote' as const,
      manual_ratio: null,
    }))
    return { ...defaultConfig, mappings }
  }
  try {
    const value = JSON.parse(raw) as Partial<NikoGroupSyncConfig>
    const savedMappings = Array.isArray(value.mappings) ? value.mappings : []
    const selectedGroups = new Set(availableGroups)
    const mappings = savedMappings.map((mapping) =>
      mapping.local_group.trim() === ''
        ? mapping
        : {
            ...mapping,
            enabled: selectedGroups.has(mapping.local_group.trim()),
          }
    )
    const mappedLocalGroups = new Set(
      mappings.map((mapping) => mapping.local_group)
    )
    for (const group of availableGroups) {
      if (mappedLocalGroups.has(group)) continue
      mappings.push({
        id: `niko-${group}`,
        enabled: true,
        local_group: group,
        remote_group: '',
        ratio_mode: 'remote',
        manual_ratio: null,
      })
    }
    return {
      ...defaultConfig,
      ...value,
      mappings,
    }
  } catch {
    return { ...defaultConfig, mappings: defaultMappings }
  }
}

function newMapping(index: number): NikoGroupMapping {
  return {
    id: `mapping-${Date.now()}-${index}`,
    enabled: true,
    local_group: '',
    remote_group: '',
    ratio_mode: 'remote',
    manual_ratio: null,
  }
}

function mappingKey(mapping: NikoGroupMapping, index: number): string {
  return mapping.id || `mapping-${index}`
}

function parseNumberMap(raw?: string): Record<string, number> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    return Object.fromEntries(
      Object.entries(parsed).filter(([, value]) => typeof value === 'number')
    ) as Record<string, number>
  } catch {
    return {}
  }
}

function parseStringMap(raw?: string): Record<string, string> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    return Object.fromEntries(
      Object.entries(parsed).filter(([, value]) => typeof value === 'string')
    ) as Record<string, string>
  } catch {
    return {}
  }
}

function configSignature(config: NikoGroupSyncConfig): string {
  return JSON.stringify({
    enabled: config.enabled,
    source_url: config.source_url,
    timeout_seconds: config.timeout_seconds,
    ratio_adjustment: config.ratio_adjustment,
    mappings: config.mappings.map((mapping) => ({
      id: mapping.id ?? '',
      enabled: mapping.enabled,
      local_group: mapping.local_group,
      remote_group: mapping.remote_group,
      ratio_mode: mapping.ratio_mode,
      manual_ratio: mapping.manual_ratio ?? null,
    })),
  })
}

export function NikoGroupSyncSettings() {
  const options = useSystemOptions()
  const queryClient = useQueryClient()
  const channelGroupsQuery = useQuery({
    queryKey: ['channel-group-list'],
    queryFn: () => getChannels({ p: 1, page_size: 1000 }),
    retry: false,
  })
  const rawConfig = useMemo(
    () =>
      options.data?.data.find((item) => item.key === 'NikoGroupSyncConfig')
        ?.value,
    [options.data]
  )
  const [draftConfig, setDraftConfig] = useState<NikoGroupSyncConfig>({
    ...defaultConfig,
    mappings: defaultMappings,
  })
  const [draftSource, setDraftSource] = useState<string>()
  const rawGroupRatio = options.data?.data.find(
    (item) => item.key === 'GroupRatio'
  )?.value
  const rawUsableGroup = options.data?.data.find(
    (item) => item.key === 'UserUsableGroups'
  )?.value
  const storedGroupRatio = useMemo(
    () => parseNumberMap(rawGroupRatio),
    [rawGroupRatio]
  )
  const storedUsableGroup = useMemo(
    () => parseStringMap(rawUsableGroup),
    [rawUsableGroup]
  )
  const savedConfig = useMemo(() => {
    const parsed = readNikoGroupSyncConfig(
      rawConfig,
      Object.keys(storedUsableGroup)
    )
    if (
      !parsed.base_group_ratio ||
      Object.keys(parsed.base_group_ratio).length === 0
    ) {
      parsed.base_group_ratio = Object.fromEntries(
        parsed.mappings
          .map((mapping) => [
            mapping.local_group,
            storedGroupRatio[mapping.local_group],
          ])
          .filter(([, ratio]) => typeof ratio === 'number')
      )
    }
    return parsed
  }, [rawConfig, storedGroupRatio, storedUsableGroup])
  const config = draftSource === rawConfig ? draftConfig : savedConfig
  const storedBaseGroupRatio = savedConfig.base_group_ratio ?? storedGroupRatio
  const [groupRatioEdits, setGroupRatioEdits] = useState<
    Record<string, number | null>
  >({})
  const updateConfig = (
    next:
      | NikoGroupSyncConfig
      | ((current: NikoGroupSyncConfig) => NikoGroupSyncConfig)
  ) => {
    setDraftSource(rawConfig)
    setDraftConfig((current) => {
      const base = draftSource === rawConfig ? current : savedConfig
      return typeof next === 'function' ? next(base) : next
    })
  }
  const [preview, setPreview] = useState<NikoGroupPreview | null>(null)
  const [previewError, setPreviewError] = useState('')
  const isDirty = useMemo(() => {
    if (configSignature(config) !== configSignature(savedConfig)) return true
    return config.mappings.some((mapping, index) => {
      const key = mappingKey(mapping, index)
      if (!(key in groupRatioEdits)) return false
      const ratio = groupRatioEdits[key]
      if (ratio == null) return mapping.local_group in storedBaseGroupRatio
      return storedBaseGroupRatio[mapping.local_group] !== ratio
    })
  }, [config, groupRatioEdits, savedConfig, storedBaseGroupRatio])
  const saveMutation = useMutation({
    mutationFn: saveNikoGroupSync,
    onSuccess: async (data) => {
      if (data.success) {
        await queryClient.invalidateQueries({ queryKey: ['system-options'] })
        setGroupRatioEdits({})
        toast.success('分组及倍率已保存')
      } else {
        toast.error(data.message || '保存失败')
      }
    },
    onError: (error: Error) => toast.error(error.message || '保存失败'),
  })

  const getCurrentRatio = (mapping: NikoGroupMapping, index: number) => {
    const key = mappingKey(mapping, index)
    if (key in groupRatioEdits) {
      return groupRatioEdits[key]
    }
    return storedBaseGroupRatio[mapping.local_group]
  }

  const getRemoteRatio = (mapping: NikoGroupMapping) =>
    preview?.group_ratio?.[mapping.remote_group]
  const remoteGroupOptions = useMemo(
    () => Object.keys(preview?.group_ratio ?? {}).sort(),
    [preview]
  )

  const updateCurrentRatio = (
    mapping: NikoGroupMapping,
    index: number,
    value: string
  ) => {
    const key = mappingKey(mapping, index)
    setGroupRatioEdits((current) => ({
      ...current,
      [key]: value === '' ? null : Number(value),
    }))
  }

  const copyRemoteMapping = (mapping: NikoGroupMapping, index: number) => {
    const remoteRatio = getRemoteRatio(mapping)
    if (remoteRatio == null) return
    updateMapping(index, { local_group: mapping.remote_group })
    updateCurrentRatio(mapping, index, String(remoteRatio))
  }

  const updateMapping = (index: number, patch: Partial<NikoGroupMapping>) => {
    updateConfig((current) => ({
      ...current,
      mappings: current.mappings.map((mapping, mappingIndex) =>
        mappingIndex === index ? { ...mapping, ...patch } : mapping
      ),
    }))
  }

  const pendingRemoteMappings = config.mappings
    .map((mapping, index) => ({ mapping, index }))
    .filter(
      ({ mapping }) =>
        !mapping.enabled &&
        mapping.local_group.trim() === '' &&
        mapping.remote_group.trim() !== '' &&
        getRemoteRatio(mapping) != null
    )

  const syncAndEnablePendingMappings = () => {
    const pendingKeys = new Set(
      pendingRemoteMappings.map(({ mapping, index }) =>
        mappingKey(mapping, index)
      )
    )
    setGroupRatioEdits((current) => {
      const next = { ...current }
      pendingRemoteMappings.forEach(({ mapping, index }) => {
        const remoteRatio = getRemoteRatio(mapping)
        if (remoteRatio != null) {
          next[mappingKey(mapping, index)] = remoteRatio
        }
      })
      return next
    })
    updateConfig((current) => ({
      ...current,
      mappings: current.mappings.map((mapping, index) =>
        pendingKeys.has(mappingKey(mapping, index))
          ? {
              ...mapping,
              enabled: true,
              local_group: mapping.remote_group,
            }
          : mapping
      ),
    }))
  }

  const deletePendingMappings = () => {
    const pendingKeys = new Set(
      pendingRemoteMappings.map(({ mapping, index }) =>
        mappingKey(mapping, index)
      )
    )
    setGroupRatioEdits((current) =>
      Object.fromEntries(
        Object.entries(current).filter(([key]) => !pendingKeys.has(key))
      )
    )
    updateConfig((current) => ({
      ...current,
      mappings: current.mappings.filter(
        (mapping, index) => !pendingKeys.has(mappingKey(mapping, index))
      ),
    }))
  }

  const save = () => {
    const mappings = (
      config.mappings.length > 0 ? config.mappings : defaultMappings
    ).map((mapping, index) => ({
      ...mapping,
      id: mapping.id || `mapping-${index + 1}`,
      local_group: mapping.local_group.trim(),
      remote_group: mapping.remote_group.trim(),
      ratio_mode: 'remote' as const,
      manual_ratio: null,
    }))
    if (
      !Number.isFinite(config.ratio_adjustment) ||
      config.ratio_adjustment < 0 ||
      config.ratio_adjustment > 10
    ) {
      toast.error('全局倍率加成必须在 0 到 10 之间')
      return
    }
    const selectedLocalGroups = new Set(Object.keys(storedUsableGroup))
    const enabledMappings = mappings.filter(
      (mapping) =>
        mapping.enabled &&
        (mapping.local_group === '' ||
          selectedLocalGroups.has(mapping.local_group))
    )
    const baseGroupRatio = { ...storedBaseGroupRatio }
    enabledMappings.forEach((mapping, index) => {
      const key = mappingKey(mapping, index)
      if (!(key in groupRatioEdits)) return
      const group = mapping.local_group
      const ratio = groupRatioEdits[key]
      if (ratio == null) delete baseGroupRatio[group]
      else baseGroupRatio[group] = ratio
    })
    const activeLocalMappings = enabledMappings.filter(
      (mapping) => mapping.local_group !== ''
    )
    const invalidRatio = activeLocalMappings.find((mapping) => {
      const ratio = baseGroupRatio[mapping.local_group]
      return ratio == null || !Number.isFinite(ratio) || ratio < 0 || ratio > 10
    })
    if (invalidRatio) {
      toast.error('当前倍率必须在 0 到 10 之间')
      return
    }
    const next = {
      ...config,
      mappings: enabledMappings,
      base_group_ratio: baseGroupRatio,
    }
    const groupRatio = { ...storedGroupRatio }
    activeLocalMappings.forEach((mapping) => {
      const baseRatio = baseGroupRatio[mapping.local_group]
      groupRatio[mapping.local_group] = Number(
        (baseRatio + config.ratio_adjustment).toFixed(10)
      )
    })
    updateConfig(next)
    saveMutation.mutate({
      config: next,
      group_ratio: groupRatio,
    })
  }

  const previewMutation = useMutation({
    mutationFn: previewNikoGroupSync,
    onSuccess: (data) => {
      if (!data.success) {
        setPreviewError(data.message || '获取倍率失败')
        return
      }
      setPreviewError('')
      if (!data.data) {
        setPreviewError('接口未返回倍率数据')
        return
      }
      setPreview(data.data)
      toast.success('已获取当前倍率')
    },
    onError: (error: Error) => setPreviewError(error.message || '获取倍率失败'),
  })
  const syncGroupsMutation = useMutation({
    mutationFn: previewNikoGroupSync,
    onSuccess: (data) => {
      if (!data.success || !data.data) {
        setPreviewError(data.message || '同步分组失败')
        return
      }
      setPreviewError('')
      setPreview(data.data)
      const remoteGroups = Object.keys(data.data.group_ratio)
      updateConfig((current) => {
        const knownGroups = new Set(
          current.mappings
            .flatMap((mapping) => [
              mapping.local_group.trim().toLowerCase(),
              mapping.remote_group.trim().toLowerCase(),
            ])
            .filter(Boolean)
        )
        const newMappings = remoteGroups
          .filter((group) => !knownGroups.has(group.trim().toLowerCase()))
          .map((group, index) => ({
            id: `niko-remote-${Date.now()}-${index}`,
            enabled: false,
            local_group: '',
            remote_group: group,
            ratio_mode: 'remote' as const,
            manual_ratio: null,
          }))
        return newMappings.length > 0
          ? { ...current, mappings: [...current.mappings, ...newMappings] }
          : current
      })
      toast.success('已同步 Niko 远程分组')
    },
    onError: (error: Error) => setPreviewError(error.message || '同步分组失败'),
  })

  return (
    <div className='flex w-full shrink-0 flex-col gap-4'>
      <FormNavigationGuard when={isDirty} />
      <SettingsCard
        title='分组倍率同步'
        description='当前倍率同步配置暂使用 Niko 作为来源。分组启用状态跟随分组定价页的用户可选设置；远程倍率只用于对比，不会自动覆盖当前倍率，也不会添加或修改模型。'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <label className='flex items-center gap-2 text-sm'>
            <input
              type='checkbox'
              checked={config.enabled}
              onChange={(event) =>
                updateConfig({ ...config, enabled: event.target.checked })
              }
            />
            启用定时检查
          </label>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='niko-timeout'>请求超时（秒，1-120）</Label>
            <Input
              id='niko-timeout'
              type='number'
              min={1}
              max={120}
              value={config.timeout_seconds}
              onChange={(event) =>
                updateConfig({
                  ...config,
                  timeout_seconds: Number(event.target.value),
                })
              }
            />
          </div>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='niko-adjustment'>
              保存时应用到所有分组的全局倍率加成（0-10）
            </Label>
            <Input
              id='niko-adjustment'
              type='number'
              min={0}
              max={10}
              step='0.01'
              value={
                Number.isNaN(config.ratio_adjustment)
                  ? ''
                  : config.ratio_adjustment
              }
              onChange={(event) =>
                updateConfig({
                  ...config,
                  ratio_adjustment:
                    event.target.value === ''
                      ? Number.NaN
                      : Number(event.target.value),
                })
              }
            />
          </div>
          <div className='flex flex-col gap-2 md:col-span-2'>
            <Label htmlFor='niko-source'>Niko 地址</Label>
            <Input
              id='niko-source'
              value={config.source_url}
              onChange={(event) =>
                updateConfig({ ...config, source_url: event.target.value })
              }
            />
          </div>
        </div>
        <div className='mt-4 flex flex-wrap gap-2'>
          {isDirty && (
            <Button
              type='button'
              onClick={save}
              disabled={saveMutation.isPending}
              aria-busy={saveMutation.isPending}
            >
              <Save />
              {saveMutation.isPending ? '保存中...' : '保存设置'}
            </Button>
          )}
          <Button
            type='button'
            variant='outline'
            onClick={() => previewMutation.mutate()}
            disabled={previewMutation.isPending}
            aria-busy={previewMutation.isPending}
          >
            <RefreshCw />
            {previewMutation.isPending ? '获取中...' : '获取当前倍率'}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() => syncGroupsMutation.mutate()}
            disabled={syncGroupsMutation.isPending}
            aria-busy={syncGroupsMutation.isPending}
          >
            <RefreshCw />
            {syncGroupsMutation.isPending ? '同步中...' : '同步分组'}
          </Button>
        </div>
        {(previewError || saveMutation.isError) && (
          <div className='text-destructive mt-3 space-y-1 text-sm' role='alert'>
            {previewError && <div>获取失败：{previewError}</div>}
            {saveMutation.isError && (
              <div>保存失败：{saveMutation.error?.message || '请求失败'}</div>
            )}
          </div>
        )}
      </SettingsCard>

      <SettingsCard
        title='分组映射'
        description='会列出本地已配置的全部分组；分组不会自动添加模型，模型请在渠道设置中自行配置。'
      >
        <div className='w-full overflow-x-auto'>
          <table className='w-max min-w-full text-sm'>
            <thead>
              <tr className='border-b text-left'>
                <th className='w-16 min-w-16 px-2 py-2 font-medium'>启用</th>
                <th className='w-56 min-w-56 px-2 py-2 font-medium'>
                  本地分组
                </th>
                <th className='w-56 min-w-56 px-2 py-2 font-medium'>
                  远端分组
                </th>
                <th className='w-40 min-w-40 px-2 py-2 font-medium'>
                  当前倍率
                </th>
                <th className='w-40 min-w-40 px-2 py-2 font-medium'>
                  远程倍率
                </th>
                <th className='w-36 min-w-36 px-2 py-2 font-medium'>操作</th>
              </tr>
            </thead>
            <tbody>
              {config.mappings.map((mapping, index) => (
                <tr
                  key={mapping.id || `mapping-${index}`}
                  className='border-b last:border-0'
                >
                  <td className='px-2 py-2 align-middle'>
                    <input
                      type='checkbox'
                      aria-label={`启用第 ${index + 1} 行`}
                      checked={mapping.enabled}
                      disabled={mapping.local_group.trim() !== ''}
                      title={
                        mapping.local_group.trim() !== ''
                          ? '启用状态跟随分组定价中的用户可选设置'
                          : undefined
                      }
                      onChange={(event) =>
                        updateMapping(index, { enabled: event.target.checked })
                      }
                    />
                  </td>
                  <td className='px-2 py-2 whitespace-nowrap'>
                    <Input
                      aria-label={`第 ${index + 1} 行本地分组`}
                      value={mapping.local_group}
                      placeholder='例如 default'
                      onChange={(event) =>
                        updateMapping(index, {
                          local_group: event.target.value,
                        })
                      }
                    />
                  </td>
                  <td className='px-2 py-2 whitespace-nowrap'>
                    <Input
                      aria-label={`第 ${index + 1} 行远端分组`}
                      list='niko-remote-group-options'
                      value={mapping.remote_group}
                      placeholder='例如 codex-Pro'
                      onChange={(event) =>
                        updateMapping(index, {
                          remote_group: event.target.value,
                        })
                      }
                    />
                  </td>
                  <td className='px-2 py-2 whitespace-nowrap'>
                    <Input
                      aria-label={`第 ${index + 1} 行当前倍率`}
                      type='number'
                      min={0}
                      max={10}
                      step='0.01'
                      value={getCurrentRatio(mapping, index) ?? ''}
                      onChange={(event) =>
                        updateCurrentRatio(mapping, index, event.target.value)
                      }
                    />
                  </td>
                  <td className='px-2 py-2 whitespace-nowrap'>
                    <span className='block py-2 font-medium'>
                      {getRemoteRatio(mapping) ?? ''}
                    </span>
                  </td>
                  <td className='px-2 py-2 whitespace-nowrap'>
                    <div className='flex items-center gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={`同步第 ${index + 1} 行远程分组和倍率`}
                        title='同步远程分组和倍率到当前行'
                        disabled={getRemoteRatio(mapping) == null}
                        onClick={() => copyRemoteMapping(mapping, index)}
                      >
                        <ArrowDownToLine />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={`删除第 ${index + 1} 行`}
                        title='删除映射'
                        onClick={() =>
                          updateConfig((current) => ({
                            ...current,
                            mappings: current.mappings.filter(
                              (_, mappingIndex) => mappingIndex !== index
                            ),
                          }))
                        }
                      >
                        <Trash2 />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {config.mappings.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className='text-muted-foreground px-2 py-6 text-center'
                  >
                    暂无映射，请新增一行
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <datalist id='niko-remote-group-options'>
          {remoteGroupOptions.map((group) => (
            <option key={group} value={group} />
          ))}
        </datalist>
        <div className='mt-3 flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            title='将待配置远程行同步到本地分组和当前倍率并启用'
            disabled={pendingRemoteMappings.length === 0}
            onClick={syncAndEnablePendingMappings}
          >
            <ArrowDownToLine />
            一键同步并启用
          </Button>
          <Button
            type='button'
            variant='outline'
            title='删除所有待配置远程行'
            disabled={pendingRemoteMappings.length === 0}
            onClick={deletePendingMappings}
          >
            <Trash2 />
            一键删除待配置分组
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              updateConfig((current) => ({
                ...current,
                mappings: [
                  ...current.mappings,
                  newMapping(current.mappings.length + 1),
                ],
              }))
            }
          >
            <Plus />
            新增映射
          </Button>
        </div>
      </SettingsCard>

      <SettingsCard
        title='渠道分组查看列表'
        description='查看当前渠道配置中的分组；此列表仅供查看，不会修改渠道或分组定价。'
      >
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>
            共 {channelGroupsQuery.data?.data?.total ?? 0} 个渠道
          </span>
          <Button
            type='button'
            variant='outline'
            size='icon'
            title='刷新渠道分组列表'
            aria-label='刷新渠道分组列表'
            onClick={() => channelGroupsQuery.refetch()}
            disabled={channelGroupsQuery.isFetching}
            aria-busy={channelGroupsQuery.isFetching}
          >
            <RefreshCw />
          </Button>
        </div>
        <div className='mt-3 w-full overflow-x-auto'>
          <table className='w-max min-w-full text-sm'>
            <thead>
              <tr className='border-b text-left'>
                <th className='min-w-20 px-2 py-2 font-medium'>渠道 ID</th>
                <th className='min-w-64 px-2 py-2 font-medium'>渠道名称</th>
                <th className='min-w-80 px-2 py-2 font-medium'>分组</th>
              </tr>
            </thead>
            <tbody>
              {channelGroupsQuery.data?.data?.items.map((channel) => (
                <tr key={channel.id} className='border-b last:border-0'>
                  <td className='px-2 py-2'>{channel.id}</td>
                  <td className='px-2 py-2'>{channel.name}</td>
                  <td className='px-2 py-2'>
                    {channel.group
                      .split(',')
                      .map((group) => group.trim())
                      .filter(Boolean)
                      .join('、') || '未配置'}
                  </td>
                </tr>
              ))}
              {!channelGroupsQuery.isFetching &&
                channelGroupsQuery.data?.data?.items.length === 0 && (
                  <tr>
                    <td
                      colSpan={3}
                      className='text-muted-foreground px-2 py-6 text-center'
                    >
                      暂无渠道分组数据
                    </td>
                  </tr>
                )}
              {channelGroupsQuery.isError && (
                <tr>
                  <td
                    colSpan={3}
                    className='text-destructive px-2 py-6 text-center'
                  >
                    渠道分组列表获取失败
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </SettingsCard>
    </div>
  )
}
