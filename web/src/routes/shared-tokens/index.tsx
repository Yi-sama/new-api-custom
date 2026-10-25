import { createFileRoute } from '@tanstack/react-router'

import { DailyTokenPoolPage } from '@/features/daily-token-pool'

export const Route = createFileRoute('/shared-tokens/')({
  component: DailyTokenPoolPage,
})
