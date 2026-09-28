import { createFileRoute } from '@tanstack/react-router'

import { ChannelStatusPage } from '@/features/channel-status'

export const Route = createFileRoute('/_authenticated/channels/status')({
  component: ChannelStatusPage,
})
