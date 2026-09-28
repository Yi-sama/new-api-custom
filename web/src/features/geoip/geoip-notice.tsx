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
import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { useNotificationStore } from '@/stores/notification-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { getVisitorCountry } from './api'

export function GeoIPNotice() {
  const { t } = useTranslation()
  const [dismissed, setDismissed] = useState(false)
  const [dontRemindToday, setDontRemindToday] = useState(false)
  const checkboxId = useId()
  const closedUntilDate = useNotificationStore(
    (state) => state.geoIPClosedUntilDate
  )
  const setClosedUntilDate = useNotificationStore(
    (state) => state.setGeoIPClosedUntilDate
  )
  const systemName = useSystemConfigStore((state) => state.config.systemName)
  const { data: country } = useQuery({
    queryKey: ['visitor-country'],
    queryFn: getVisitorCountry,
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  })

  return (
    <Dialog
      open={
        country === 'CN' &&
        !dismissed &&
        closedUntilDate !== new Date().toDateString()
      }
      onOpenChange={(open) => {
        if (open) return
        setDismissed(true)
        if (dontRemindToday) {
          try {
            setClosedUntilDate(new Date().toDateString())
          } catch {
            // Closing still works when browser storage is unavailable.
          }
        }
      }}
    >
      <DialogContent
        className='max-h-[calc(100dvh-2rem)] overflow-y-auto rounded-lg sm:max-w-lg'
        showCloseButton={false}
      >
        <DialogHeader className='gap-5 pr-8'>
          <DialogTitle className='text-lg leading-normal font-semibold'>
            {t('GeoIP access notice')}
          </DialogTitle>
          <DialogDescription className='text-base leading-7 break-words'>
            {t(
              'You have reached {{siteName}}. Based on your IP location and our service region policy, this service is not available in your region.',
              { siteName: systemName }
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogClose
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              className='absolute top-2 right-2'
              aria-label={t('Close')}
            />
          }
        >
          <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} />
        </DialogClose>
        <DialogFooter className='mt-2 flex-col gap-4 rounded-b-lg sm:items-center sm:justify-between'>
          <div className='flex items-center gap-2'>
            <Checkbox
              id={checkboxId}
              checked={dontRemindToday}
              onCheckedChange={setDontRemindToday}
            />
            <Label htmlFor={checkboxId}>{t("Don't remind me today")}</Label>
          </div>
          <DialogClose render={<Button className='min-w-28' />}>
            {t('I understand')}
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
