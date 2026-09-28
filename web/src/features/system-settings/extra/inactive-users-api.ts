import { api } from '@/lib/api'

export type InactiveUser = {
  id: number
  username: string
  display_name: string
  email: string
  created_at: number
  last_api_call_at: number
  api_activity_started_at: number
  inactive_days: number
}

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}

export type InactiveUsersSettings = {
  days: number
}

export type InactiveUsersPage = {
  items: InactiveUser[]
  total: number
  page: number
  page_size: number
  cutoff: number
  observation_count: number
}

export type InactiveUsersDeleteResult = {
  deleted: number
  skipped: number
  failed: number
}

export type DeleteInactiveUsersRequest = {
  days: number
  cutoff: number
  confirmed: true
} & ({ ids: number[] } | { all: true })

function unwrap<T>(response: ApiEnvelope<T>, fallbackMessage: string): T {
  if (!response.success) {
    throw new Error(response.message || fallbackMessage)
  }

  return response.data
}

export async function getInactiveUsersSettings(): Promise<InactiveUsersSettings> {
  const response = await api.get<ApiEnvelope<InactiveUsersSettings>>(
    '/api/inactive-users/settings'
  )
  return unwrap(response.data, 'Failed to load inactive-user settings.')
}

export async function saveInactiveUsersSettings(
  days: number
): Promise<InactiveUsersSettings> {
  const response = await api.put<ApiEnvelope<InactiveUsersSettings>>(
    '/api/inactive-users/settings',
    { days }
  )
  return unwrap(response.data, 'Failed to save inactive-user settings.')
}

export async function getInactiveUsers(request: {
  days: number
  page: number
  pageSize: number
}): Promise<InactiveUsersPage> {
  const response = await api.get<ApiEnvelope<InactiveUsersPage>>(
    '/api/inactive-users',
    {
      params: {
        days: request.days,
        page: request.page,
        page_size: request.pageSize,
      },
    }
  )
  return unwrap(response.data, 'Failed to load inactive accounts.')
}

export async function deleteInactiveUsers(
  request: DeleteInactiveUsersRequest
): Promise<InactiveUsersDeleteResult> {
  const response = await api.post<ApiEnvelope<InactiveUsersDeleteResult>>(
    '/api/inactive-users/delete',
    request
  )
  return unwrap(response.data, 'Failed to clean inactive accounts.')
}
