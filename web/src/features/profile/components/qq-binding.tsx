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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Copy, KeyRound, Loader2, RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { createQQBindingCode, getQQBinding } from '../api'

const schema = z.object({ qq: z.string().regex(/^[1-9][0-9]{4,11}$/) })
const errors: Record<string, string> = {
  QQ_BENEFIT_NEED_START:
    'Send /领福利 in the QQ group before generating a binding code.',
  QQ_BENEFIT_ALREADY_BOUND: 'This QQ or site account is already bound.',
  QQ_BENEFIT_SESSION_INVALID: 'Sign in again before binding your QQ account.',
  QQ_BENEFIT_INVALID: 'Enter a valid QQ number.',
}

export function QQBinding(props: { userId: number; onUpdate: () => void }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const notified = useRef(false)
  const onUpdate = props.onUpdate
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { qq: '' },
  })
  const generate = useMutation({
    onMutate: () => form.clearErrors('root'),
    onError: (error: Error) =>
      form.setError('root', { message: error.message }),
    mutationFn: async (values: z.infer<typeof schema>) => {
      const response = await createQQBindingCode(values.qq)
      if (!response.success || !response.data) {
        throw new Error(
          errors[response.code || ''] ||
            'QQ binding is unavailable. Please try again.'
        )
      }
      return response.data
    },
  })
  const binding = useQuery({
    queryKey: ['profile', 'qq-binding', props.userId],
    queryFn: async () => {
      const response = await getQQBinding()
      if (!response.success || !response.data) {
        throw new Error('QQ binding is unavailable. Please try again.')
      }
      return response.data
    },
    refetchInterval: (query) =>
      open &&
      generate.data &&
      generate.data.expires_at * 1000 > Date.now() &&
      !query.state.data?.bound
        ? 2000
        : false,
  })
  useEffect(() => {
    if (!binding.data?.bound || !generate.data || notified.current) return
    notified.current = true
    toast.success(t('QQ binding completed.'))
    onUpdate()
  }, [binding.data?.bound, generate.data, onUpdate, t])

  if (binding.data?.enabled === false) return null

  const code = generate.data?.code || ''

  return (
    <div className='mb-4 border-b pb-4'>
      <div className='flex min-w-0 items-center justify-between gap-3'>
        <div className='min-w-0'>
          <p className='text-sm font-medium'>{t('QQ Binding')}</p>
          <p className='text-muted-foreground text-xs break-words'>
            {binding.data?.bound
              ? t('QQ {{qq}} · First-binding credit received', {
                  qq: binding.data.qq,
                })
              : t('First binding: $1 API credit')}
          </p>
          {binding.isError && (
            <p role='alert' className='text-destructive mt-1 text-xs'>
              {t('QQ binding is unavailable. Please try again.')}
            </p>
          )}
        </div>
        {binding.isError ? (
          <Button
            variant='outline'
            size='icon'
            title={t('Refresh')}
            aria-label={t('Refresh')}
            onClick={() => binding.refetch()}
          >
            <RefreshCw className='size-4' />
          </Button>
        ) : (
          <Button
            variant='outline'
            size='sm'
            disabled={binding.isPending || binding.data?.bound}
            onClick={() => setOpen(true)}
          >
            <KeyRound className='size-4' />
            {binding.data?.bound ? t('Bound') : t('Bind QQ')}
          </Button>
        )}
      </div>
      <Dialog
        open={open && !binding.data?.bound}
        onOpenChange={(value) => {
          setOpen(value)
          if (!value) {
            form.reset()
            generate.reset()
          }
        }}
        title={t('Bind QQ')}
        description={t('First binding: $1 API credit')}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4'
      >
        <form
          onSubmit={form.handleSubmit((values) => generate.mutate(values))}
          className='space-y-4 py-4'
        >
          <p className='text-muted-foreground text-sm'>
            {t(
              'Send /领福利 in the QQ group before generating a binding code.'
            )}
          </p>
          <div className='space-y-2'>
            <Label htmlFor='qq-binding-number'>{t('QQ Number')}</Label>
            <Input
              id='qq-binding-number'
              inputMode='numeric'
              autoComplete='off'
              maxLength={12}
              disabled={generate.isPending}
              aria-invalid={Boolean(form.formState.errors.qq)}
              aria-describedby={
                form.formState.errors.qq ? 'qq-binding-error' : undefined
              }
              {...form.register('qq', {
                onChange: () => {
                  generate.reset()
                  form.clearErrors('root')
                },
              })}
            />
            {form.formState.errors.qq && (
              <p
                id='qq-binding-error'
                role='alert'
                className='text-destructive text-sm'
              >
                {t('Enter a valid QQ number.')}
              </p>
            )}
          </div>
          {form.formState.errors.root?.message && (
            <p role='alert' className='text-destructive text-sm break-words'>
              {t(form.formState.errors.root.message)}
            </p>
          )}
          <Button
            type='submit'
            disabled={generate.isPending}
            className='w-full'
          >
            {generate.isPending ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <KeyRound className='size-4' />
            )}
            {t('Generate binding code')}
          </Button>
          {generate.data && (
            <div className='space-y-2 border-t pt-4' role='status'>
              <p className='text-sm'>
                {t('QQ {{qq}}', { qq: generate.data.qq })}
              </p>
              <div className='flex items-center justify-between gap-3'>
                <code className='min-w-0 text-lg break-all'>{code}</code>
                <Button
                  type='button'
                  size='icon'
                  variant='outline'
                  title={t('Copy binding code')}
                  aria-label={t('Copy binding code')}
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(code)
                      toast.success(t('Copied'))
                    } catch {
                      toast.error(t('Copy failed'))
                    }
                  }}
                >
                  <Copy className='size-4' />
                </Button>
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('Valid until {{time}}', {
                  time: new Date(
                    generate.data.expires_at * 1000
                  ).toLocaleTimeString(),
                })}
              </p>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Send this code from the same QQ in the group to finish binding.'
                )}
              </p>
            </div>
          )}
        </form>
      </Dialog>
    </div>
  )
}
