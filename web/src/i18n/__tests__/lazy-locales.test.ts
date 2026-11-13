import { expect, it, vi } from 'vitest'

it('loads uncommon languages only when selected and preserves their translations', async () => {
  vi.resetModules()
  const { default: i18n } = await import('../config')
  await i18n.changeLanguage('en')

  expect(i18n.hasResourceBundle('en', 'translation')).toBe(true)
  expect(i18n.hasResourceBundle('zhCN', 'translation')).toBe(true)
  for (const language of ['fr', 'ru', 'ja', 'vi', 'zhTW']) {
    expect(i18n.hasResourceBundle(language, 'translation')).toBe(false)
  }

  for (const language of ['fr', 'ru', 'ja', 'vi', 'zhTW']) {
    await i18n.changeLanguage(language)
    expect(i18n.hasResourceBundle(language, 'translation')).toBe(true)
    expect(i18n.t('Studio parameter profiles')).not.toBe(
      'Studio parameter profiles'
    )
  }

  await i18n.changeLanguage('en')
  expect(i18n.t('My Liandong Shop')).toBe('My Liandong Shop')
})
