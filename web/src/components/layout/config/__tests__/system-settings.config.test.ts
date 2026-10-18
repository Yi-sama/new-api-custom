import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import { SYSTEM_SETTINGS_VIEW } from '../system-settings.config'

describe('system settings navigation', () => {
  it('places Fast group settings only under extra settings', () => {
    const t = ((key: string) => key) as TFunction
    const items = SYSTEM_SETTINGS_VIEW.getNavGroups(t)[0].items
    const operations = items.find((item) => item.title === 'Operations')
    const extra = items.find((item) => item.title === '额外设置')
    expect(operations?.items?.map((item) => item.url)).not.toContain(
      '/system-settings/operations/fast-mode'
    )
    expect(extra?.items).toContainEqual({
      title: 'Fast group settings',
      url: '/system-settings/operations/fast-mode',
    })
  })
  it('places status images only under extra settings', () => {
    const t = ((key: string) => key) as TFunction
    const items = SYSTEM_SETTINGS_VIEW.getNavGroups(t)[0].items
    const operations = items.find((item) => item.title === 'Operations')
    const extra = items.find((item) => item.title === '额外设置')
    expect(operations?.items?.map((item) => item.url)).not.toContain(
      '/system-settings/operations/status-images'
    )
    expect(extra?.items).toContainEqual({
      title: 'Status images',
      url: '/system-settings/operations/status-images',
    })
  })
  it('places the Auto administrator entry only under extra settings', () => {
    const t = ((key: string) => key) as TFunction
    const items = SYSTEM_SETTINGS_VIEW.getNavGroups(t)[0].items
    const operations = items.find((item) => item.title === 'Operations')
    const extra = items.find((item) => item.title === '额外设置')

    expect(operations?.items?.map((item) => item.url)).not.toContain(
      '/system-settings/operations/auto-model'
    )
    expect(extra?.items).toContainEqual({
      title: 'Auto 默认设置',
      url: '/system-settings/operations/auto-model',
    })
  })

  it('places inactive API account cleanup only under extra settings', () => {
    const t = ((key: string) => key) as TFunction
    const items = SYSTEM_SETTINGS_VIEW.getNavGroups(t)[0].items
    const operations = items.find((item) => item.title === 'Operations')
    const extra = items.find((item) => item.title === '额外设置')

    expect(operations?.items?.map((item) => item.url)).not.toContain(
      '/system-settings/operations/inactive-users'
    )
    expect(extra?.items).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          title: 'Inactive API account cleanup',
          url: '/system-settings/operations/inactive-users',
        }),
      ])
    )
  })
})
