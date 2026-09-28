import { api } from '@/lib/api'

export type Ticket = {
  id: number
  ticket_no: string
  user_id: number
  username?: string
  title: string
  status: 'open' | 'closed'
  last_read_at: number
  closed_at: number
  created_at: number
  updated_at: number
  awaiting_user?: boolean
  reply_status?: string
}

export type TicketMessage = {
  id: number
  ticket_id: number
  sender_id: number
  sender_type: 'user' | 'admin'
  content: string
  created_at: number
}

export type TicketDetail = { ticket: Ticket; messages: TicketMessage[] }

export type TicketPage = {
  page: number
  page_size: number
  total: number
  items: Ticket[]
}

export type TicketListParams = {
  status?: string
  replyStatus?: string
  keyword?: string
  awaitingUser?: string
  sortBy?: string
  sortOrder?: string
}

export async function listTickets(
  admin = false,
  params: TicketListParams = {}
): Promise<TicketPage> {
  const query = new URLSearchParams()
  if (params.status) query.set('status', params.status)
  if (params.replyStatus) query.set('reply_status', params.replyStatus)
  if (params.keyword) query.set('keyword', params.keyword)
  if (params.awaitingUser) query.set('awaiting_user', params.awaitingUser)
  if (params.sortBy) query.set('sort_by', params.sortBy)
  if (params.sortOrder) query.set('sort_order', params.sortOrder)
  const suffix = query.toString() ? `?${query.toString()}` : ''
  const response = await api.get<{ success: boolean; data: TicketPage }>(
    `${admin ? '/api/tickets/admin' : '/api/tickets'}${suffix}`
  )
  return response.data.data
}

export async function createTicket(
  title: string,
  content: string
): Promise<Ticket> {
  const response = await api.post<{ success: boolean; data: Ticket }>(
    '/api/tickets',
    {
      title,
      content,
    }
  )
  return response.data.data
}

export async function getTicket(id: string): Promise<TicketDetail> {
  const response = await api.get<{ success: boolean; data: TicketDetail }>(
    `/api/tickets/${id}`
  )
  return response.data.data
}

export async function addTicketMessage(
  id: string,
  content: string
): Promise<TicketMessage> {
  const response = await api.post<{ success: boolean; data: TicketMessage }>(
    `/api/tickets/${id}/messages`,
    { content }
  )
  return response.data.data
}

export async function closeTicket(id: string): Promise<void> {
  await api.post(`/api/tickets/${id}/close`)
}

export async function deleteTicket(id: string): Promise<void> {
  await api.delete(`/api/tickets/${id}`)
}

export async function deleteClosedTickets(): Promise<number> {
  const response = await api.delete<{
    success: boolean
    data: { deleted: number }
  }>('/api/tickets/closed')
  return response.data.data.deleted
}
