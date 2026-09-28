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

type ImageStudioSessionResponse = {
  success: boolean
  message?: string
  data?: { url?: string }
}

export async function createImageStudioSession(): Promise<string> {
  const response = await api.post<ImageStudioSessionResponse>(
    '/api/image-studio/session',
    undefined,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  const url = response.data.data?.url
  if (!response.data.success || typeof url !== 'string' || !url.trim()) {
    throw new Error(response.data.message || 'Invalid Image Studio session')
  }
  const protocol = new URL(url, window.location.origin).protocol
  if (protocol !== 'http:' && protocol !== 'https:') {
    throw new Error('Invalid Image Studio URL')
  }
  return url
}
