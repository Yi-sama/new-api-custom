import { z } from 'zod'

export const STATUS_IMAGE_OPTION = 'StatusImageConfig'

export const statusImageSchema = z.object({
  site_name: z.string().trim().min(1).max(60),
  channel_title: z.string().trim().min(1).max(60),
  sub_title: z.string().trim().min(1).max(60),
  footer: z.string().max(160),
  group_filter: z.string().max(500),
  channel_columns: z.union([z.literal(1), z.literal(2)]),
  show_success_rate: z.boolean(),
  show_latency: z.boolean(),
  show_history: z.boolean(),
  show_updated_at: z.boolean(),
  show_sub_details: z.boolean(),
})

export type StatusImageConfig = z.infer<typeof statusImageSchema>
export type ReportKind = 'channel' | 'sub'

export const defaultStatusImageConfig: StatusImageConfig = {
  site_name: 'Yisama的自用站',
  channel_title: '渠道状态',
  sub_title: 'Sub2API 状态',
  footer: '',
  group_filter: '',
  channel_columns: 2,
  show_success_rate: true,
  show_latency: true,
  show_history: true,
  show_updated_at: true,
  show_sub_details: true,
}
