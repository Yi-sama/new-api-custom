import { Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'

import type { LogOtherData } from '../types'

export function FastModeBadge(props: {
  other: LogOtherData | null
}): React.JSX.Element | null {
  const { t } = useTranslation()
  if (props.other?.fast_mode !== true) return null

  const ratio = props.other.fast_mode_ratio
  const hasRatio = ratio != null && Number.isFinite(ratio) && ratio > 0
  const chargedFast = hasRatio && ratio > 1
  let label = t('Fast requested')
  if (chargedFast) {
    label = t('Fast {{ratio}}x', { ratio })
  } else if (hasRatio) {
    label = t('Fast · standard {{ratio}}x', { ratio })
  }

  return (
    <StatusBadge
      label={label}
      icon={Zap}
      variant={chargedFast ? 'amber' : 'neutral'}
      copyable={false}
      type='badge'
      className='h-5 max-w-full rounded-md border border-current/20 text-xs [&_span]:text-xs'
    />
  )
}
