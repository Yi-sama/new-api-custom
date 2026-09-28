import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Search, Save, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getEnabledModels } from '@/features/channels/api'

import { updateSystemOption } from '../api'
import { SettingsCard } from '../components/settings-card'
import type { OperationsSettings } from '../types'

type AutoModelSettingsProps = Pick<OperationsSettings, 'AutoModelDefaultModels'>

export function AutoModelSettings(props: AutoModelSettingsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const modelsQuery = useQuery({
    queryKey: ['enabled-models-for-auto-default'],
    queryFn: getEnabledModels,
    retry: false,
  })
  const [draftModels, setDraftModels] = useState<string[] | null>(null)
  const [search, setSearch] = useState('')

  const enabledModels = useMemo(
    () => modelsQuery.data?.data ?? [],
    [modelsQuery.data?.data]
  )
  const savedModels =
    props.AutoModelDefaultModels.length > 0
      ? props.AutoModelDefaultModels
      : enabledModels
  const models = draftModels ?? savedModels
  const updateModels = (updater: (current: string[]) => string[]) => {
    setDraftModels((current) => updater(current ?? models))
  }

  const availableModels = useMemo(() => {
    return [...new Set([...models, ...enabledModels])]
  }, [enabledModels, models])

  const visibleModels = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return availableModels
    return availableModels.filter((model) =>
      model.toLowerCase().includes(keyword)
    )
  }, [availableModels, search])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const defaultModels = JSON.stringify(models)
      const defaultResponse = await updateSystemOption({
        key: 'AutoModelDefaultModels',
        value: defaultModels,
      })
      if (!defaultResponse.success) {
        throw new Error(defaultResponse.message || t('保存失败'))
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Auto 默认设置已保存'))
    },
    onError: (error: Error) => toast.error(error.message || t('保存失败')),
  })

  const moveModel = (index: number, offset: -1 | 1) => {
    const target = index + offset
    if (target < 0 || target >= models.length) return
    updateModels((current) => {
      const next = [...current]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  return (
    <SettingsCard
      title={t('Auto 默认设置')}
      description={t('管理 Auto 的站内默认模型列表。')}
    >
      <div className='space-y-5'>
        <section className='space-y-3'>
          <div className='flex items-center justify-between gap-3'>
            <div>
              <Label>{t('站内默认 Auto 模型')}</Label>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('用户未自定义时，将按这里的列表配置 Auto。')}
              </p>
            </div>
            <span className='text-muted-foreground text-xs'>
              {models.length} / {availableModels.length}
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
                  aria-label={t('上移')}
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
                  aria-label={t('下移')}
                >
                  <ArrowDown className='size-3.5' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  onClick={() =>
                    updateModels((current) =>
                      current.filter((item) => item !== model)
                    )
                  }
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
          <Label>{t('添加默认模型')}</Label>
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
                  onCheckedChange={(checked) => {
                    updateModels((current) =>
                      checked === true
                        ? [...current, model]
                        : current.filter((item) => item !== model)
                    )
                  }}
                />
                <span className='min-w-0 truncate font-mono text-xs'>
                  {model}
                </span>
              </label>
            ))}
          </div>
        </section>

        <Button
          type='button'
          onClick={() => saveMutation.mutate()}
          disabled={models.length === 0 || saveMutation.isPending}
        >
          <Save />
          {saveMutation.isPending ? t('保存中...') : t('保存 Auto 默认设置')}
        </Button>
      </div>
    </SettingsCard>
  )
}
