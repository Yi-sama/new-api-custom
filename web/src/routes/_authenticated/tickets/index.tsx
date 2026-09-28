import { createFileRoute } from '@tanstack/react-router'

import { TicketListPage } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/tickets/')({
  component: TicketListPage,
})
