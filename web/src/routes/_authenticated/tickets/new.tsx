import { createFileRoute } from '@tanstack/react-router'

import { TicketCreatePage } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/tickets/new')({
  component: TicketCreatePage,
})
