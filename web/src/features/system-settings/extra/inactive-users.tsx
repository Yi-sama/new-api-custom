import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  Save,
  Trash2,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { SettingsCard } from '../components/settings-card'
import {
  deleteInactiveUsers,
  getInactiveUsers,
  getInactiveUsersSettings,
  saveInactiveUsersSettings,
  type InactiveUser,
} from './inactive-users-api'

const PAGE_SIZE = 20

type PendingDeletion =
  | {
      type: 'single'
      days: number
      cutoff: number
      user: InactiveUser
    }
  | {
      type: 'all'
      days: number
      cutoff: number
      count: number
    }

function parseInactiveDays(value: string): number | null {
  if (!/^\d+$/.test(value.trim())) return null

  const days = Number(value)
  if (!Number.isSafeInteger(days) || days < 1 || days > 3650) return null

  return days
}

function formatUnixTime(value: number): string | null {
  if (!value) return null
  return new Date(value * 1000).toLocaleString()
}

export function InactiveUsersSettings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const initializedSettings = useRef(false)
  const [daysInput, setDaysInput] = useState('')
  const [appliedDays, setAppliedDays] = useState<number | null>(null)
  const [page, setPage] = useState(1)
  const [pendingDeletion, setPendingDeletion] =
    useState<PendingDeletion | null>(null)
  const [cleanupFeedback, setCleanupFeedback] = useState<{
    tone: 'error' | 'status'
    message: string
  } | null>(null)

  const settingsQuery = useQuery({
    queryKey: ['inactive-users-settings'],
    queryFn: getInactiveUsersSettings,
    retry: false,
  })

  useEffect(() => {
    if (!settingsQuery.data || initializedSettings.current) return

    initializedSettings.current = true
    setDaysInput(String(settingsQuery.data.days))
    setAppliedDays(settingsQuery.data.days)
  }, [settingsQuery.data])

  const parsedDays = parseInactiveDays(daysInput)
  const hasUnappliedDays = parsedDays === null || parsedDays !== appliedDays

  const inactiveUsersQuery = useQuery({
    queryKey: ['inactive-users', appliedDays, page],
    queryFn: () =>
      getInactiveUsers({ days: appliedDays ?? 1, page, pageSize: PAGE_SIZE }),
    enabled: appliedDays !== null,
    retry: false,
  })

  const saveSettingsMutation = useMutation({
    mutationFn: (days: number) => saveInactiveUsersSettings(days),
    onSuccess: async (_settings, days) => {
      setAppliedDays(days)
      setPage(1)
      setCleanupFeedback(null)
      queryClient.setQueryData(['inactive-users-settings'], { days })
      await queryClient.invalidateQueries({ queryKey: ['inactive-users'] })
      toast.success(t('Inactive-user settings saved.'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save inactive-user settings.'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (deletion: PendingDeletion) => {
      if (deletion.type === 'single') {
        return deleteInactiveUsers({
          days: deletion.days,
          cutoff: deletion.cutoff,
          confirmed: true,
          ids: [deletion.user.id],
        })
      }

      return deleteInactiveUsers({
        days: deletion.days,
        cutoff: deletion.cutoff,
        confirmed: true,
        all: true,
      })
    },
    onSuccess: (result) => {
      setCleanupFeedback({
        tone: result.failed > 0 ? 'error' : 'status',
        message: t(
          'Deleted {{deleted}} account(s), skipped {{skipped}}, failed {{failed}}.',
          result
        ),
      })
      setPendingDeletion(null)
      setPage(1)
    },
    onError: (error: Error) => {
      setCleanupFeedback({
        tone: 'error',
        message: error.message || t('Failed to clean inactive accounts.'),
      })
    },
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['inactive-users'] })
    },
  })

  const listData = inactiveUsersQuery.data
  const pageCount = Math.max(
    1,
    Math.ceil((listData?.total ?? 0) / (listData?.page_size ?? PAGE_SIZE))
  )
  const deletionDisabled =
    hasUnappliedDays ||
    !listData ||
    inactiveUsersQuery.isFetching ||
    saveSettingsMutation.isPending ||
    deleteMutation.isPending

  const openSingleDeletion = (user: InactiveUser) => {
    if (deletionDisabled || appliedDays === null || !listData) return

    setPendingDeletion({
      type: 'single',
      days: appliedDays,
      cutoff: listData.cutoff,
      user,
    })
  }

  const openAllDeletion = () => {
    if (deletionDisabled || appliedDays === null || !listData) return

    setPendingDeletion({
      type: 'all',
      days: appliedDays,
      cutoff: listData.cutoff,
      count: listData.total,
    })
  }

  if (settingsQuery.isPending && !settingsQuery.data) {
    return (
      <SettingsCard title={t('Inactive API account cleanup')}>
        <p className='text-muted-foreground text-sm'>
          {t('Loading inactive-user settings...')}
        </p>
      </SettingsCard>
    )
  }

  if (settingsQuery.isError) {
    return (
      <SettingsCard title={t('Inactive API account cleanup')}>
        <div className='flex flex-wrap items-center gap-3'>
          <p className='text-destructive text-sm' role='alert'>
            {settingsQuery.error.message ||
              t('Could not load inactive-user settings.')}
          </p>
          <Button
            type='button'
            variant='outline'
            onClick={() => settingsQuery.refetch()}
          >
            <RefreshCw />
            {t('Retry')}
          </Button>
        </div>
      </SettingsCard>
    )
  }

  return (
    <SettingsCard title={t('Inactive API account cleanup')}>
      <div className='space-y-5'>
        <div className='flex flex-wrap items-end gap-3'>
          <div className='space-y-2'>
            <Label htmlFor='inactive-user-days'>
              {t('Inactivity threshold (days)')}
            </Label>
            <Input
              id='inactive-user-days'
              type='number'
              min={1}
              max={3650}
              step={1}
              disabled={deleteMutation.isPending}
              value={daysInput}
              onChange={(event) => setDaysInput(event.target.value)}
              aria-invalid={parsedDays === null}
              className='w-44'
            />
            {parsedDays === null && (
              <p className='text-destructive text-xs' role='alert'>
                {t('Enter a whole number from 1 to 3650.')}
              </p>
            )}
          </div>
          <Button
            type='button'
            onClick={() => {
              if (parsedDays !== null) saveSettingsMutation.mutate(parsedDays)
            }}
            disabled={
              parsedDays === null ||
              !hasUnappliedDays ||
              saveSettingsMutation.isPending ||
              deleteMutation.isPending
            }
          >
            <Save />
            {saveSettingsMutation.isPending
              ? t('Saving...')
              : t('Apply threshold and refresh')}
          </Button>
        </div>

        {hasUnappliedDays && appliedDays !== null && (
          <p className='text-muted-foreground text-xs'>
            {t('Saved threshold is stale. Apply and refresh before deleting.')}
          </p>
        )}

        <div className='space-y-1 text-sm'>
          <p>{t('Only eligible non-administrator users are shown.')}</p>
          <p className='text-muted-foreground'>
            {t(
              'Accounts with no recorded API call are not proven to have never called the API.'
            )}
          </p>
          {listData && (
            <p className='text-muted-foreground'>
              {t(
                'Observing {{count}} ordinary account(s) with insufficient or unknown API history.',
                { count: listData.observation_count }
              )}
            </p>
          )}
        </div>

        {cleanupFeedback && (
          <p
            className={
              cleanupFeedback.tone === 'error'
                ? 'text-destructive text-sm'
                : 'text-sm'
            }
            role={cleanupFeedback.tone === 'error' ? 'alert' : 'status'}
          >
            {cleanupFeedback.message}
          </p>
        )}

        {inactiveUsersQuery.isPending && !listData && (
          <p className='text-muted-foreground text-sm'>
            {t('Loading inactive accounts...')}
          </p>
        )}

        {inactiveUsersQuery.isError && (
          <div className='flex flex-wrap items-center gap-3'>
            <p className='text-destructive text-sm' role='alert'>
              {inactiveUsersQuery.error.message ||
                t('Could not load inactive accounts.')}
            </p>
            <Button
              type='button'
              variant='outline'
              onClick={() => inactiveUsersQuery.refetch()}
            >
              <RefreshCw />
              {t('Retry')}
            </Button>
          </div>
        )}

        {listData && !inactiveUsersQuery.isError && (
          <div className='space-y-3'>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <p className='text-muted-foreground text-sm'>
                {t('{{count}} account(s) match the current threshold.', {
                  count: listData.total,
                })}
              </p>
              <Button
                type='button'
                variant='destructive'
                onClick={openAllDeletion}
                disabled={deletionDisabled || listData.total === 0}
              >
                <Trash2 />
                {t('Delete all {{count}} matching accounts', {
                  count: listData.total,
                })}
              </Button>
            </div>

            {listData.items.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('No eligible inactive accounts found.')}
              </p>
            ) : (
              <Table className='min-w-[960px]'>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Username')}</TableHead>
                    <TableHead>{t('Created')}</TableHead>
                    <TableHead>{t('Last API call')}</TableHead>
                    <TableHead>{t('API observation started')}</TableHead>
                    <TableHead>{t('Inactive days')}</TableHead>
                    <TableHead className='w-12'>
                      <span className='sr-only'>{t('Actions')}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {listData.items.map((user) => (
                    <TableRow key={user.id}>
                      <TableCell className='whitespace-normal'>
                        <div className='max-w-64 font-medium break-all'>
                          {user.username}
                        </div>
                        {(user.display_name || user.email) && (
                          <div className='text-muted-foreground max-w-64 text-xs break-all'>
                            {[user.display_name, user.email]
                              .filter(Boolean)
                              .join(' · ')}
                          </div>
                        )}
                      </TableCell>
                      <TableCell>
                        {formatUnixTime(user.created_at) ?? t('Unknown')}
                      </TableCell>
                      <TableCell>
                        {user.last_api_call_at
                          ? formatUnixTime(user.last_api_call_at)
                          : t('No calls observed')}
                      </TableCell>
                      <TableCell>
                        {formatUnixTime(user.api_activity_started_at) ??
                          t('Unknown')}
                      </TableCell>
                      <TableCell>{user.inactive_days}</TableCell>
                      <TableCell>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          onClick={() => openSingleDeletion(user)}
                          disabled={deletionDisabled}
                          title={t('Delete account {{username}}', {
                            username: user.username,
                          })}
                          aria-label={t('Delete account {{username}}', {
                            username: user.username,
                          })}
                        >
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <div className='flex items-center justify-end gap-2'>
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={() => setPage((current) => Math.max(1, current - 1))}
                disabled={listData.page <= 1 || inactiveUsersQuery.isFetching}
                aria-label={t('Go to previous page')}
              >
                <ChevronLeft />
              </Button>
              <span
                className='text-muted-foreground text-sm'
                aria-live='polite'
              >
                {t('Page {{page}} of {{pages}}', {
                  page: listData.page,
                  pages: pageCount,
                })}
              </span>
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={() =>
                  setPage((current) => Math.min(pageCount, current + 1))
                }
                disabled={
                  listData.page >= pageCount || inactiveUsersQuery.isFetching
                }
                aria-label={t('Go to next page')}
              >
                <ChevronRight />
              </Button>
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={pendingDeletion !== null}
        onOpenChange={(open) => {
          if (!open && !deleteMutation.isPending) setPendingDeletion(null)
        }}
        title={
          pendingDeletion?.type === 'single'
            ? t('Delete account {{username}}', {
                username: pendingDeletion.user.username,
              })
            : t('Delete {{count}} matching inactive accounts?', {
                count: pendingDeletion?.count ?? 0,
              })
        }
        desc={
          <div className='space-y-2'>
            <p>
              {t('This cleanup will permanently delete {{count}} account(s).', {
                count:
                  pendingDeletion?.type === 'single'
                    ? 1
                    : (pendingDeletion?.count ?? 0),
              })}
            </p>
            <p>
              {t(
                'The {{days}}-day threshold and current cutoff are captured for this irreversible cleanup.',
                { days: pendingDeletion?.days ?? appliedDays ?? 0 }
              )}
            </p>
            {pendingDeletion && (
              <p>
                {t('Cutoff: {{cutoff}}', {
                  cutoff:
                    formatUnixTime(pendingDeletion.cutoff) ?? t('Unknown'),
                })}
              </p>
            )}
          </div>
        }
        destructive
        confirmText={
          deleteMutation.isPending ? t('Deleting...') : t('Delete permanently')
        }
        isLoading={deleteMutation.isPending}
        disabled={deletionDisabled}
        handleConfirm={() => {
          if (pendingDeletion && !deletionDisabled) {
            deleteMutation.mutate(pendingDeletion)
          }
        }}
      >
        {deleteMutation.isError && (
          <p role='alert' className='text-destructive text-sm'>
            {deleteMutation.error.message ||
              t('Failed to clean inactive accounts.')}
          </p>
        )}
      </ConfirmDialog>
    </SettingsCard>
  )
}
