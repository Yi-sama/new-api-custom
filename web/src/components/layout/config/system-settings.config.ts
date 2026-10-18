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
import type { TFunction } from 'i18next'
import {
  Box,
  CreditCard,
  Layout,
  Settings,
  Shield,
  ShieldAlert,
  UserRoundX,
  Wrench,
  RefreshCw,
} from 'lucide-react'

import { getAuthSectionNavItems } from '@/features/system-settings/auth/section-registry.tsx'
import { getBillingSectionNavItems } from '@/features/system-settings/billing/section-registry.tsx'
import { getContentSectionNavItems } from '@/features/system-settings/content/section-registry.tsx'
import { getModelsSectionNavItems } from '@/features/system-settings/models/section-registry.tsx'
import { getOperationsSectionNavItems } from '@/features/system-settings/operations/section-registry.tsx'
import { getSecuritySectionNavItems } from '@/features/system-settings/security/section-registry.tsx'
import { getSiteSectionNavItems } from '@/features/system-settings/site/section-registry.tsx'

import type { NavGroup, SidebarView } from '../types'

/**
 * Sidebar nav groups for the System Settings nested view.
 *
 * Kept as a single group because the workspace title in the sidebar
 * header already provides top-level context — the inner group label
 * scopes the items as "administration" actions.
 */
function getSystemSettingsNavGroups(t: TFunction): NavGroup[] {
  return [
    {
      id: 'system-administration',
      title: t('System Administration'),
      items: [
        {
          title: t('Site & Branding'),
          icon: Settings,
          items: getSiteSectionNavItems(t),
        },
        {
          title: t('Authentication'),
          icon: Shield,
          items: getAuthSectionNavItems(t),
        },
        {
          title: t('Billing & Payment'),
          icon: CreditCard,
          items: getBillingSectionNavItems(t),
        },
        {
          title: t('Models & Routing'),
          icon: Box,
          items: getModelsSectionNavItems(t),
        },
        {
          title: t('Security & Limits'),
          icon: ShieldAlert,
          items: getSecuritySectionNavItems(t),
        },
        {
          title: t('Console Content'),
          icon: Layout,
          items: getContentSectionNavItems(t),
        },
        {
          title: t('Operations'),
          icon: Wrench,
          items: getOperationsSectionNavItems(t).filter(
            (item) =>
              ![
                '/system-settings/operations/auto-model',
                '/system-settings/operations/fast-mode',
                '/system-settings/operations/niko-group-sync',
                '/system-settings/operations/channel-health',
                '/system-settings/operations/channel-status',
                '/system-settings/operations/inactive-users',
                '/system-settings/operations/status-images',
              ].includes(item.url)
          ),
        },
        {
          title: '额外设置',
          icon: RefreshCw,
          items: [
            {
              title: 'Auto 默认设置',
              url: '/system-settings/operations/auto-model',
            },
            {
              title: t('Fast group settings'),
              url: '/system-settings/operations/fast-mode',
            },
            {
              title: '分组倍率同步',
              url: '/system-settings/operations/niko-group-sync',
            },
            {
              title: '渠道健康监控',
              url: '/system-settings/operations/channel-health',
            },
            {
              title: '渠道状态',
              url: '/system-settings/operations/channel-status',
            },
            {
              title: t('Inactive API account cleanup'),
              icon: UserRoundX,
              url: '/system-settings/operations/inactive-users',
            },
            {
              title: t('Status images'),
              url: '/system-settings/operations/status-images',
            },
          ],
        },
      ],
    },
  ]
}

/**
 * Nested sidebar view for `/system-settings/*`.
 *
 * Activates the Vercel / Cloudflare-style drill-in sidebar:
 * the root navigation is replaced by the system administration
 * groups, with a "Back to Dashboard" affordance in the header.
 */
export const SYSTEM_SETTINGS_VIEW: SidebarView = {
  id: 'system-settings',
  pathPattern: /^\/system-settings(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getSystemSettingsNavGroups,
}
