import { expect, it } from 'vitest'

import { convertDetectedLanguage } from '../languages'

it('preserves the saved traditional Chinese language on the next page load', () => {
  expect(convertDetectedLanguage('zhTW')).toBe('zhTW')
})
