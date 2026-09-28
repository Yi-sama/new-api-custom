import assert from 'node:assert/strict'
import { mkdir } from 'node:fs/promises'
import { createRequire } from 'node:module'
import path from 'node:path'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const baseUrl = process.env.HOME_TEST_URL || 'http://127.0.0.1:3001'
const output = process.env.HOME_SCREENSHOTS
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.HOME_BROWSER_PATH || undefined,
})
const page = await browser.newPage()
const pageErrors = []
page.on('pageerror', (error) => pageErrors.push(error.message))
await page.addInitScript(() => localStorage.setItem('i18nextLng', 'zhCN'))

async function checkLayout(width, height) {
  await page.setViewportSize({ width, height })
  await page.locator('.home-title').waitFor()
  await page.evaluate(() => document.fonts.ready)
  const layout = await page.evaluate(() => {
    const rect = (selector) => {
      const r = document.querySelector(selector).getBoundingClientRect()
      return { left: r.left, right: r.right, top: r.top, bottom: r.bottom }
    }
    return {
      width: innerWidth,
      height: innerHeight,
      scrollWidth: document.documentElement.scrollWidth,
      scrollHeight: document.documentElement.scrollHeight,
      intro: rect('.home-intro'),
      models: rect('.home-models'),
      community: rect('.home-community'),
      footer: rect('footer'),
      header: rect('header'),
      brokenImages: [...document.images].filter(
        (image) => !image.complete || image.naturalWidth === 0
      ).length,
    }
  })
  assert.ok(layout.scrollHeight <= height + 1, JSON.stringify(layout))
  assert.ok(layout.scrollWidth <= width, JSON.stringify(layout))
  assert.ok(layout.intro.top >= layout.header.bottom, JSON.stringify(layout))
  assert.ok(layout.intro.bottom <= layout.footer.top, JSON.stringify(layout))
  assert.ok(
    layout.community.bottom <= layout.footer.top,
    JSON.stringify(layout)
  )
  assert.ok(layout.models.top >= layout.intro.bottom, JSON.stringify(layout))
  assert.ok(layout.community.top >= layout.intro.bottom, JSON.stringify(layout))
  assert.ok(
    width < 640
      ? layout.community.top >= layout.models.bottom
      : layout.community.left >= layout.models.right,
    JSON.stringify(layout)
  )
  assert.equal(layout.brokenImages, 0)
  assert.equal(
    await page.locator('.home-model-upcoming .home-model-caption').isVisible(),
    true
  )
  assert.equal(await page.locator('.home-qr img').isVisible(), true)
  assert.match(
    await page.locator('.home-community-copy').innerText(),
    /1125690897/
  )
  console.log(`PASS layout ${width}x${height}`)
}

try {
  await page.goto(baseUrl, { waitUntil: 'networkidle' })
  for (const [width, height] of [
    [1920, 1080],
    [1440, 900],
    [1366, 768],
    [1024, 768],
    [768, 1024],
    [390, 844],
    [375, 667],
    [320, 568],
    [844, 390],
  ]) {
    await checkLayout(width, height)
  }

  if (output) {
    await mkdir(output, { recursive: true })
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.screenshot({ path: path.join(output, 'home-niko-desktop.png') })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.screenshot({ path: path.join(output, 'home-niko-mobile.png') })
  }

  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: '切换导航菜单' }).click()
  await page.getByRole('button', { name: '切换导航菜单' }).click()
  await checkLayout(390, 844)

  await page.setViewportSize({ width: 1440, height: 900 })
  await page.getByRole('button', { name: '切换主题' }).click()
  await page.getByRole('menuitem', { name: '深色', exact: true }).click()
  await page.getByRole('menu').waitFor({ state: 'hidden' })
  await page.waitForFunction(() =>
    document.documentElement.classList.contains('dark')
  )
  await checkLayout(1440, 900)
  if (output) {
    await page.screenshot({ path: path.join(output, 'home-niko-dark.png') })
  }

  await page.locator('.home-console').focus()
  await page.keyboard.press('Enter')
  await page.waitForURL('**/sign-in?**')
  assert.equal(new URL(page.url()).searchParams.get('redirect'), '/dashboard')
  await page.goto(baseUrl, { waitUntil: 'networkidle' })
  await page.locator('.home-status').click()
  await page.waitForURL('**/sign-in?**')
  assert.equal(
    new URL(page.url()).searchParams.get('redirect'),
    '/channels/status'
  )

  // Extra catalog entries must not turn the homepage preview into availability claims.
  await page.route('**/api/pricing', (route) =>
    route.fulfill({
      json: {
        success: true,
        data: ['gpt-test', 'claude-test', 'gemini-test', 'deepseek-test'].map(
          (model_name) => ({ model_name })
        ),
      },
    })
  )
  await page.goto(baseUrl, { waitUntil: 'networkidle' })
  await page.waitForFunction(
    () => document.querySelectorAll('.home-model').length === 2
  )
  await checkLayout(320, 568)
  await checkLayout(844, 390)
  assert.deepEqual(pageErrors, [])
  console.log(
    'PASS dark theme, mobile menu, keyboard entry, auth redirects, upcoming models and QR image'
  )
} finally {
  await browser.close()
}
