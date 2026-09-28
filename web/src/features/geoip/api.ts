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

export async function getVisitorCountry(): Promise<string | null> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 5000)

  try {
    const response = await fetch('/cdn-cgi/trace', {
      cache: 'no-store',
      credentials: 'omit',
      redirect: 'error',
      signal: controller.signal,
    })
    if (
      !response.ok ||
      !response.headers.get('content-type')?.startsWith('text/plain')
    ) {
      return null
    }

    const trace = await response.text()
    const country = trace.match(/^loc=([A-Z]{2})\r?$/m)?.[1]
    return country && country !== 'XX' ? country : null
  } catch {
    return null
  } finally {
    clearTimeout(timeout)
  }
}
