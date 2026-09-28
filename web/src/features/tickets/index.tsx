import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  CheckCircle2,
  Clock3,
  Inbox,
  MessageSquare,
  Plus,
  Search,
  Send,
  Trash2,
  UserRound,
  X,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  addTicketMessage,
  closeTicket,
  createTicket,
  deleteClosedTickets,
  deleteTicket,
  getTicket,
  listTickets,
} from './api'

function formatTime(timestamp: number) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : ''
}

function getMessageSenderLabel(
  senderType: string,
  isAdmin: boolean,
  username: string | undefined,
  translate: (key: string) => string
) {
  if (senderType === 'admin') return translate('Administrator')
  if (isAdmin) return username || translate('Unknown user')
  return translate('You')
}

function TicketStatusBadge({
  ticket,
  isAdmin = false,
}: {
  ticket: { status: string; awaiting_user?: boolean; reply_status?: string }
  isAdmin?: boolean
}) {
  const { t } = useTranslation()
  if (isAdmin && ticket.reply_status) {
    switch (ticket.reply_status) {
      case 'unreplied':
        return (
          <Badge variant='warning'>
            <Clock3 /> {t('Unreplied')}
          </Badge>
        )
      case 'replied':
        return (
          <Badge className='border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300'>
            <CheckCircle2 /> {t('Replied')}
          </Badge>
        )
      case 'followup_unreplied':
        return (
          <Badge variant='warning'>
            <Clock3 /> {t('Follow-up unanswered')}
          </Badge>
        )
      case 'followup_replied':
        return (
          <Badge className='border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-300'>
            <MessageSquare /> {t('Follow-up answered')}
          </Badge>
        )
      case 'closed':
        return (
          <Badge variant='secondary'>
            <CheckCircle2 /> {t('Closed')}
          </Badge>
        )
    }
  }
  if (ticket.awaiting_user) {
    return (
      <Badge variant='warning'>
        <Clock3 /> {t('Waiting for user')}
      </Badge>
    )
  }
  if (ticket.status === 'open') {
    return (
      <Badge className='border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-300'>
        <Inbox /> {t('Open')}
      </Badge>
    )
  }
  return (
    <Badge variant='secondary'>
      <CheckCircle2 /> {t('Closed')}
    </Badge>
  )
}

function EmptyTickets() {
  const { t } = useTranslation()
  return (
    <div className='border-border/60 bg-muted/10 flex min-h-48 flex-col items-center justify-center rounded-xl border border-dashed px-6 text-center'>
      <MessageSquare className='text-muted-foreground mb-3 size-8' />
      <p className='font-medium'>{t('No tickets yet')}</p>
      <p className='text-muted-foreground mt-1 text-sm'>
        {t('Create a ticket and track its replies')}
      </p>
    </div>
  )
}

export function TicketCreatePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const createMutation = useMutation({
    mutationFn: () => createTicket(title, content),
    onSuccess: () => {
      setTitle('')
      setContent('')
      toast.success(t('Ticket created'))
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
      void navigate({ to: '/tickets' })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      ),
  })

  return (
    <main className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-4xl flex-col gap-5 p-4 md:p-8'>
        <header className='flex flex-wrap items-end justify-between gap-4'>
          <div>
            <Link
              to='/tickets'
              className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-sm transition-colors'
            >
              <ArrowLeft className='size-4' /> {t('Back to tickets')}
            </Link>
            <h1 className='mt-3 text-3xl font-semibold tracking-tight'>
              {t('New ticket')}
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Describe your issue')}
            </p>
          </div>
        </header>
        <section className='border-border/60 bg-card rounded-xl border p-5 shadow-sm md:p-8'>
          <div className='grid gap-5'>
            <Input
              className='h-12 text-base'
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder={t('Title')}
              maxLength={120}
            />
            <Textarea
              className='min-h-72 resize-y text-base'
              value={content}
              onChange={(event) => setContent(event.target.value)}
              placeholder={t('Describe your issue')}
              maxLength={10000}
            />
            <div className='flex flex-wrap justify-end gap-3'>
              <Button variant='outline' render={<Link to='/tickets' />}>
                {t('Cancel')}
              </Button>
              <Button
                disabled={
                  !title.trim() || !content.trim() || createMutation.isPending
                }
                onClick={() => createMutation.mutate()}
              >
                <Send /> {t('Submit ticket')}
              </Button>
            </div>
          </div>
        </section>
      </div>
    </main>
  )
}

export function TicketListPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role ?? ROLE.USER)
  const isAdmin = role >= ROLE.ADMIN
  const [replyStatus, setReplyStatus] = useState('')
  const [keyword, setKeyword] = useState('')
  const [sortOrder, setSortOrder] = useState('desc')
  const query = useQuery({
    queryKey: ['tickets', isAdmin, replyStatus, keyword, sortOrder],
    queryFn: () =>
      listTickets(isAdmin, {
        replyStatus,
        keyword,
        sortBy: 'updated_at',
        sortOrder,
      }),
  })
  const deleteClosedMutation = useMutation({
    mutationFn: deleteClosedTickets,
    onSuccess: (deleted) => {
      toast.success(t('Deleted {{count}} closed tickets', { count: deleted }))
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      ),
  })
  const deleteMutation = useMutation({
    mutationFn: deleteTicket,
    onSuccess: () => {
      toast.success(t('Ticket deleted'))
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      ),
  })

  return (
    <main className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-6xl flex-col gap-5 p-4 md:p-8'>
        <header className='flex flex-wrap items-end justify-between gap-4'>
          <div>
            <p className='text-muted-foreground mb-1 text-sm font-medium'>
              {t('Support')}
            </p>
            <h1 className='text-3xl font-semibold tracking-tight'>
              {t('Tickets')}
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>
              {isAdmin
                ? t('Review and respond to all tickets')
                : t('Create a ticket and track its replies')}
            </p>
          </div>
          {!isAdmin && (
            <Button render={<Link to='/tickets/new' />}>
              <Plus /> {t('New ticket')}
            </Button>
          )}
        </header>

        {isAdmin && (
          <section className='border-border/60 bg-card rounded-xl border p-3 shadow-sm md:p-4'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-center'>
              <div className='relative min-w-0 flex-1'>
                <Search className='text-muted-foreground absolute top-3 left-3 size-4' />
                <Input
                  className='h-10 pl-9'
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder={t('Search tickets')}
                />
              </div>
              <select
                aria-label={t('Reply state')}
                className='border-input bg-background h-10 rounded-md border px-3 text-sm'
                value={replyStatus}
                onChange={(event) => setReplyStatus(event.target.value)}
              >
                <option value=''>{t('All statuses')}</option>
                <option value='unreplied'>{t('Unreplied')}</option>
                <option value='replied'>{t('Replied')}</option>
                <option value='followup_unreplied'>
                  {t('Follow-up unanswered')}
                </option>
                <option value='followup_replied'>
                  {t('Follow-up answered')}
                </option>
                <option value='closed'>{t('Closed')}</option>
              </select>
              <select
                aria-label={t('Sort order')}
                className='border-input bg-background h-10 rounded-md border px-3 text-sm'
                value={sortOrder}
                onChange={(event) => setSortOrder(event.target.value)}
              >
                <option value='desc'>{t('Newest first')}</option>
                <option value='asc'>{t('Oldest first')}</option>
              </select>
              <Button
                variant='destructive'
                size='sm'
                onClick={() => {
                  if (window.confirm(t('Delete all closed tickets?'))) {
                    deleteClosedMutation.mutate()
                  }
                }}
                disabled={deleteClosedMutation.isPending}
              >
                <Trash2 /> {t('Delete closed')}
              </Button>
            </div>
          </section>
        )}

        <section className='space-y-2'>
          {query.isLoading && (
            <p className='text-muted-foreground px-2 py-8 text-center text-sm'>
              {t('Loading...')}
            </p>
          )}
          {query.isError && (
            <p className='text-destructive px-2 py-8 text-center text-sm'>
              {t('Failed to load tickets')}
            </p>
          )}
          {!query.isLoading &&
            !query.isError &&
            query.data?.items.length === 0 && <EmptyTickets />}
          {query.data?.items.map((ticket) => (
            <div
              key={ticket.id}
              className='border-border/60 bg-card hover:border-border flex items-center gap-3 rounded-lg border px-4 py-3 transition-colors md:px-5'
            >
              <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-full'>
                <MessageSquare className='size-4' />
              </div>
              <Link
                to='/tickets/$ticketId'
                params={{ ticketId: String(ticket.id) }}
                className='min-w-0 flex-1'
              >
                <div className='flex flex-wrap items-center gap-x-3 gap-y-1'>
                  <span className='truncate font-medium'>{ticket.title}</span>
                  <TicketStatusBadge ticket={ticket} isAdmin={isAdmin} />
                </div>
                <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-3 text-xs'>
                  <span>
                    {ticket.username ||
                      (isAdmin ? t('Unknown user') : t('You'))}
                  </span>
                  <span>{ticket.ticket_no}</span>
                  <span>{formatTime(ticket.updated_at)}</span>
                </div>
              </Link>
              {isAdmin && ticket.status === 'closed' && (
                <Button
                  variant='ghost'
                  size='icon'
                  aria-label={t('Delete ticket')}
                  onClick={() => {
                    if (window.confirm(t('Delete this ticket?'))) {
                      deleteMutation.mutate(String(ticket.id))
                    }
                  }}
                >
                  <Trash2 />
                </Button>
              )}
            </div>
          ))}
        </section>
      </div>
    </main>
  )
}

export function TicketDetailPage(props: { ticketId: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role ?? ROLE.USER)
  const isAdmin = role >= ROLE.ADMIN
  const [content, setContent] = useState('')
  const [messageOrder, setMessageOrder] = useState('desc')
  const messagesContainerRef = useRef<HTMLDivElement>(null)
  const query = useQuery({
    queryKey: ['ticket', props.ticketId],
    queryFn: () => getTicket(props.ticketId),
  })
  const messageMutation = useMutation({
    mutationFn: () => addTicketMessage(props.ticketId, content),
    onSuccess: () => {
      setContent('')
      void queryClient.invalidateQueries({
        queryKey: ['ticket', props.ticketId],
      })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      ),
  })
  const closeMutation = useMutation({
    mutationFn: () => closeTicket(props.ticketId),
    onSuccess: () => {
      toast.success(t('Ticket closed'))
      void queryClient.invalidateQueries({
        queryKey: ['ticket', props.ticketId],
      })
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      ),
  })
  const messages = query.data
    ? [...query.data.messages].sort((a, b) => a.id - b.id)
    : []
  useEffect(() => {
    const container = messagesContainerRef.current
    if (!container) return
    container.scrollTop = messageOrder === 'desc' ? container.scrollHeight : 0
  }, [messageOrder, messages.length])

  if (query.isLoading) return <main className='p-8'>{t('Loading...')}</main>
  if (query.isError || !query.data) {
    return <main className='text-destructive p-8'>{t('Ticket not found')}</main>
  }
  const ticket = query.data.ticket
  return (
    <main className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <div className='mx-auto flex min-h-0 w-full max-w-6xl flex-1 flex-col p-4 md:p-8'>
        <header className='mb-6 flex shrink-0 flex-wrap items-start justify-between gap-4'>
          <div className='min-w-0'>
            <Link
              to='/tickets'
              className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-sm transition-colors'
            >
              <ArrowLeft className='size-4' /> {t('Back to tickets')}
            </Link>
            <h1 className='mt-3 truncate text-2xl font-semibold md:text-3xl'>
              {ticket.title}
            </h1>
            <div className='mt-2 flex flex-wrap items-center gap-2'>
              <TicketStatusBadge ticket={ticket} isAdmin={isAdmin} />
              <span className='text-muted-foreground text-xs'>
                {ticket.ticket_no}
              </span>
            </div>
          </div>
          {isAdmin && ticket.status === 'open' && (
            <Button
              variant='destructive'
              onClick={() => closeMutation.mutate()}
              disabled={closeMutation.isPending}
            >
              <X /> {t('Close ticket')}
            </Button>
          )}
        </header>
        <div className='grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_auto] gap-6 overflow-hidden lg:grid-cols-[minmax(0,1fr)_280px] lg:grid-rows-1'>
          <div className='flex min-h-0 min-w-0 flex-col gap-3'>
            <div className='flex shrink-0 items-center justify-between gap-3'>
              <h2 className='text-base font-semibold'>{t('Conversation')}</h2>
              {messages.length >= 3 && (
                <select
                  aria-label={t('Message order')}
                  className='border-input bg-background h-9 rounded-md border px-3 text-sm'
                  value={messageOrder}
                  onChange={(event) => setMessageOrder(event.target.value)}
                >
                  <option value='desc'>{t('Newest first')}</option>
                  <option value='asc'>{t('Oldest first')}</option>
                </select>
              )}
            </div>
            <div
              ref={messagesContainerRef}
              className={cn(
                'min-h-0 flex-1 space-y-3 overflow-y-auto pr-1',
                messages.length >= 3 && 'max-h-[min(28rem,calc(100svh-20rem))]'
              )}
            >
              {messages.map((message) => (
                <article
                  key={message.id}
                  className='border-border/60 bg-card rounded-xl border p-4 shadow-sm md:p-5'
                >
                  <div className='flex items-center justify-between gap-3'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <div
                        className={
                          message.sender_type === 'admin'
                            ? 'bg-primary text-primary-foreground flex size-8 shrink-0 items-center justify-center rounded-full'
                            : 'bg-muted flex size-8 shrink-0 items-center justify-center rounded-full'
                        }
                      >
                        <UserRound className='size-4' />
                      </div>
                      <span className='truncate text-sm font-medium'>
                        {getMessageSenderLabel(
                          message.sender_type,
                          isAdmin,
                          ticket.username,
                          t
                        )}
                      </span>
                    </div>
                    <time className='text-muted-foreground shrink-0 text-xs'>
                      {formatTime(message.created_at)}
                    </time>
                  </div>
                  <p className='mt-4 text-sm leading-6 break-words whitespace-pre-wrap'>
                    {message.content}
                  </p>
                </article>
              ))}
            </div>
            {ticket.status === 'open' ? (
              <section className='border-border/60 bg-card shrink-0 rounded-xl border p-4 shadow-sm md:p-5'>
                <h2 className='mb-3 text-base font-semibold'>
                  {isAdmin ? t('Reply') : t('Add more information')}
                </h2>
                <Textarea
                  className='min-h-36 resize-y'
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                  placeholder={
                    isAdmin ? t('Write a reply') : t('Add more information')
                  }
                  maxLength={10000}
                />
                <div className='mt-3 flex justify-end'>
                  <Button
                    disabled={!content.trim() || messageMutation.isPending}
                    onClick={() => messageMutation.mutate()}
                  >
                    <Send /> {isAdmin ? t('Reply') : t('Add information')}
                  </Button>
                </div>
              </section>
            ) : (
              <div className='border-border/60 bg-muted/20 text-muted-foreground shrink-0 rounded-xl border px-4 py-5 text-center text-sm'>
                {t(
                  'This ticket is closed. Create a new ticket for further help.'
                )}
              </div>
            )}
          </div>
          <aside className='border-border/60 bg-card min-h-0 overflow-y-auto rounded-xl border p-5 shadow-sm'>
            <h2 className='text-base font-semibold'>{t('Ticket overview')}</h2>
            <dl className='mt-4 space-y-4 text-sm'>
              <div className='flex items-start justify-between gap-3'>
                <dt className='text-muted-foreground'>{t('Ticket ID')}</dt>
                <dd className='font-medium'>#{ticket.id}</dd>
              </div>
              <div className='flex items-start justify-between gap-3'>
                <dt className='text-muted-foreground'>{t('Author')}</dt>
                <dd className='font-medium'>
                  {ticket.username || t('Unknown user')}
                </dd>
              </div>
              <div className='flex items-start justify-between gap-3'>
                <dt className='text-muted-foreground'>{t('Replies')}</dt>
                <dd className='font-medium'>{messages.length}</dd>
              </div>
              <div className='border-border/60 border-t pt-4'>
                <dt className='text-muted-foreground'>{t('Created time')}</dt>
                <dd className='mt-1 font-medium'>
                  {formatTime(ticket.created_at)}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Last updated')}</dt>
                <dd className='mt-1 font-medium'>
                  {formatTime(ticket.updated_at)}
                </dd>
              </div>
            </dl>
          </aside>
        </div>
      </div>
    </main>
  )
}
