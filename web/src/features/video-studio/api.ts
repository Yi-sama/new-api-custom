/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

export type VideoStudioSession = {
  user_id: number
  username: string
  token: string
  expires_at: number
}

type VideoStudioSessionResponse = {
  success: boolean
  message?: string
  data?: VideoStudioSession
}

export async function createVideoStudioSession(): Promise<VideoStudioSession> {
  const response = await api.post<VideoStudioSessionResponse>(
    '/api/video-studio/session',
    undefined,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  const session = response.data.data
  if (
    !response.data.success ||
    !session ||
    typeof session.user_id !== 'number' ||
    typeof session.username !== 'string' ||
    typeof session.token !== 'string' ||
    !session.token ||
    typeof session.expires_at !== 'number'
  ) {
    throw new Error(response.data.message || 'Invalid Video Studio session')
  }
  return session
}
