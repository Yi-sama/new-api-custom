/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowUp,
  RotateCcw,
  Search,
  Sparkles,
  X,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { getAutoModelSetting, updateAutoModelSetting } from '../api'
import type { AutoModelSetting, AutoModelStrategy } from '../types'

export function AutoModelConfigurator(props: { group?: string }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['auto-model-setting', props.group || 'auto'],
    queryFn: () => getAutoModelSetting(props.group),
  })

  if (query.isLoading) {
    return <div className='bg-muted/40 h-40 animate-pulse rounded-lg' />
  }
  if (query.isError) {
    return (
      <div className='space-y-3 rounded-lg border p-4 text-center'>
        <p className='text-muted-foreground text-sm'>{t('加载失败')}</p>
        <Button type='button' variant='outline' onClick={() => query.refetch()}>
          {t('重试')}
        </Button>
      </div>
    )
  }
  if (!query.data) {
    return null
  }

  return (
    <AutoModelForm
      key={props.group || 'auto'}
      setting={query.data}
      group={props.group}
    />
  )
}

function AutoModelForm(props: { setting: AutoModelSetting; group?: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [models, setModels] = useState<string[]>(props.setting.models)
  const [strategy, setStrategy] = useState<AutoModelStrategy>(
    props.setting.strategy
  )
  const [inheritDefault, setInheritDefault] = useState(!props.setting.custom)
  const [search, setSearch] = useState('')
  const defaultModels = props.setting.default_models
  const modelsEqualDefault =
    models.length === defaultModels.length &&
    models.every((model, index) => model === defaultModels[index])
  const showRestore =
    !inheritDefault || strategy !== 'price' || !modelsEqualDefault

  const visibleModels = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return props.setting.allowed_models
    return props.setting.allowed_models.filter((model) =>
      model.toLowerCase().includes(keyword)
    )
  }, [props.setting.allowed_models, search])

  const saveMutation = useMutation({
    mutationFn: updateAutoModelSetting,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['auto-model-setting'] })
      toast.success(t('Auto 模型设置已保存'))
    },
  })
  const toggleModel = (modelName: string) => {
    setInheritDefault(false)
    setModels((current) =>
      current.includes(modelName)
        ? current.filter((item) => item !== modelName)
        : [...current, modelName]
    )
  }

  const moveModel = (index: number, offset: -1 | 1) => {
    const target = index + offset
    if (target < 0 || target >= models.length) return
    setInheritDefault(false)
    setModels((current) => {
      const next = [...current]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  const useSiteDefault = () => {
    setModels(defaultModels)
    setInheritDefault(true)
    saveMutation.mutate({
      models: defaultModels,
      strategy,
      inherit_default: true,
      ...(props.group ? { group: props.group } : {}),
    })
  }

  return (
    <div className='space-y-5'>
      <div className='flex items-start gap-3 border-b pb-4'>
        <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-lg'>
          <Sparkles className='size-5' />
        </div>
        <div className='min-w-0'>
          <h2 className='font-mono text-lg font-semibold'>Auto</h2>
          <p className='text-muted-foreground mt-1 text-sm leading-relaxed'>
            {t(
              '你可以自由选择 Auto 要使用的模型，并调整模型顺序。Auto 会根据你选择的策略，在可用模型之间自动尝试。'
            )}
          </p>
        </div>
      </div>

      <section className='space-y-3'>
        <h3 className='text-sm font-semibold'>{t('选择策略')}</h3>
        <div className='bg-muted/60 grid grid-cols-2 gap-1 rounded-lg border p-1'>
          {(['price', 'order'] as const).map((value) => {
            const description =
              value === 'price'
                ? t(
                    '优先选择预计倍率较低的模型，当前模型失败后再尝试其他可用模型。'
                  )
                : t(
                    '按照你排列的模型顺序选择，当前模型失败后再尝试下一个模型。'
                  )
            const button = (
              <button
                type='button'
                aria-pressed={strategy === value}
                onClick={() => setStrategy(value)}
                className={cn(
                  'rounded-md px-3 py-2 text-sm transition-colors',
                  strategy === value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {value === 'price' ? t('低倍率优先') : t('模型顺序优先')}
              </button>
            )

            return (
              <Tooltip key={value}>
                <TooltipTrigger render={button} />
                <TooltipContent side='bottom' className='max-w-72 text-xs'>
                  {description}
                </TooltipContent>
              </Tooltip>
            )
          })}
        </div>
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {strategy === 'price'
            ? t(
                '优先选择预计倍率较低的模型，当前模型失败后再尝试其他可用模型。'
              )
            : t('按照你排列的模型顺序选择，当前模型失败后再尝试下一个模型。')}
        </p>
      </section>

      <div className='flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'>
        <div className='min-w-0'>
          <p className='text-sm font-medium'>{t('站点默认模型列表')}</p>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('点击后立即恢复并使用管理员配置的默认模型。')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          disabled={
            saveMutation.isPending || (inheritDefault && modelsEqualDefault)
          }
          onClick={useSiteDefault}
        >
          <RotateCcw className='size-4' />
          {inheritDefault && modelsEqualDefault
            ? t('正在使用站点默认')
            : t('使用站点默认')}
        </Button>
      </div>

      <section className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <h3 className='text-sm font-semibold'>{t('已选模型')}</h3>
          <span className='text-muted-foreground text-xs'>
            {models.length} / {props.setting.allowed_models.length}
          </span>
        </div>
        <div className='max-h-56 space-y-1 overflow-y-auto rounded-lg border p-2'>
          {models.map((model, index) => (
            <div
              key={model}
              className='bg-muted/30 flex min-h-9 items-center gap-2 rounded-md px-2'
            >
              <span className='min-w-0 flex-1 truncate font-mono text-xs'>
                {model}
              </span>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                disabled={index === 0}
                onClick={() => moveModel(index, -1)}
                title={t('上移')}
              >
                <ArrowUp className='size-3.5' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                disabled={index === models.length - 1}
                onClick={() => moveModel(index, 1)}
                title={t('下移')}
              >
                <ArrowDown className='size-3.5' />
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                onClick={() => {
                  setInheritDefault(false)
                  setModels((current) =>
                    current.filter((item) => item !== model)
                  )
                }}
                title={t('移除模型')}
                aria-label={t('移除模型')}
              >
                <X className='size-3.5' />
              </Button>
            </div>
          ))}
          {models.length === 0 && (
            <p className='text-muted-foreground py-5 text-center text-sm'>
              {t('请至少选择一个 Auto 模型')}
            </p>
          )}
        </div>
      </section>

      <section className='space-y-3'>
        <h3 className='text-sm font-semibold'>{t('全部可选模型')}</h3>
        <div className='relative'>
          <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('搜索模型...')}
            className='pl-8'
          />
        </div>
        <div className='max-h-64 overflow-y-auto rounded-lg border'>
          {visibleModels.map((model) => (
            <label
              key={model}
              className='hover:bg-muted/40 flex min-h-10 items-center gap-3 border-b px-3 last:border-b-0'
            >
              <Checkbox
                checked={models.includes(model)}
                onCheckedChange={() => toggleModel(model)}
              />
              <span className='min-w-0 truncate font-mono text-xs'>
                {model}
              </span>
            </label>
          ))}
        </div>
      </section>

      <div className='flex flex-wrap justify-end gap-2 border-t pt-4'>
        {showRestore && (
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              setModels(defaultModels)
              setStrategy('price')
              setInheritDefault(true)
            }}
          >
            {t('恢复默认')}
          </Button>
        )}
        <Button
          type='button'
          disabled={
            (!inheritDefault && models.length === 0) || saveMutation.isPending
          }
          onClick={() =>
            saveMutation.mutate({
              models,
              strategy,
              inherit_default: inheritDefault,
              ...(props.group ? { group: props.group } : {}),
            })
          }
        >
          {t('保存我的 Auto')}
        </Button>
      </div>
    </div>
  )
}
