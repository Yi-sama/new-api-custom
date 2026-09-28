import { createFileRoute } from '@tanstack/react-router'

import { TicketDetailPage } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/tickets/$ticketId')({
  component: () => <TicketDetailPage ticketId={Route.useParams().ticketId} />,
})
