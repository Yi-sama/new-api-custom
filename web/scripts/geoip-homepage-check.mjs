// Run with LOCAL_GEOIP_COUNTRY=CN and Playwright supplied through NODE_PATH.
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'

const { chromium } = createRequire(import.meta.url)('playwright')

async function main() {
  const browser = await chromium.launch({ channel: 'msedge', headless: true })
  const base = 'http://localhost:3000'
  const checks = []
  try {
    const context = await browser.newContext({ locale: 'zh-CN' })
    const page = await context.newPage()
    const errors = []
    let lookups = 0
    page.on('pageerror', (error) => errors.push(error.message))
    page.on('request', (request) => {
      if (new URL(request.url()).pathname === '/cdn-cgi/trace') lookups++
    })
    const dialog = page.getByRole('dialog', { name: /GeoIP/ })

    for (const pathname of ['/sign-in', '/pricing', '/about']) {
      lookups = 0
      await page.goto(`${base}${pathname}`)
      await page.waitForLoadState('networkidle')
      assert.equal(new URL(page.url()).pathname, pathname)
      assert.equal(await dialog.count(), 0, `No notice on ${pathname}`)
      assert.equal(lookups, 0, `No GeoIP lookup on ${pathname}`)
      checks.push(`Direct visit to ${pathname}: no notice or GeoIP request`)
    }

    await page.goto(`${base}/?geoip-homepage-check=1`)
    await dialog.waitFor()
    await page.getByRole('button', { name: '我知道了' }).click()
    await dialog.waitFor({ state: 'hidden' })
    await page.locator('a[href="/pricing"]').first().click()
    await page.waitForURL('**/pricing')
    await page.waitForLoadState('networkidle')
    assert.equal(await dialog.count(), 0)
    await page.locator('a[href="/"]').first().click()
    await page.waitForURL(`${base}/`)
    await dialog.waitFor()
    checks.push(
      'Home with query string shows notice; client navigation scopes it to home'
    )

    await page.getByRole('checkbox', { name: '今天不再提醒' }).check()
    await page.getByRole('button', { name: '我知道了' }).click()
    await dialog.waitFor({ state: 'hidden' })
    await page.locator('a[href="/pricing"]').first().click()
    await page.waitForURL('**/pricing')
    await page.locator('a[href="/"]').first().click()
    await page.waitForURL(`${base}/`)
    await page.waitForLoadState('networkidle')
    assert.equal(await dialog.count(), 0)
    await page.reload()
    await page.waitForLoadState('networkidle')
    assert.equal(await dialog.count(), 0)
    checks.push(
      'Daily dismissal survives leaving home, returning, and reloading'
    )

    const signedIn = await browser.newContext({ locale: 'zh-CN' })
    const authenticatedPage = await signedIn.newPage()
    let authenticatedLookups = 0
    authenticatedPage.on('pageerror', (error) => errors.push(error.message))
    authenticatedPage.on('request', (request) => {
      if (new URL(request.url()).pathname === '/cdn-cgi/trace') {
        authenticatedLookups++
      }
    })
    await authenticatedPage.route('**/api/**', async (route) => {
      const pathname = new URL(route.request().url()).pathname
      let data = {}
      if (pathname === '/api/user/auth/refresh') {
        const now = Math.floor(Date.now() / 1000)
        data = {
          access_token: 'browser-fixture-only',
          token_type: 'Bearer',
          access_expires_at: now + 3600,
          user: {
            id: 1,
            username: 'fixture',
            display_name: 'Fixture',
            role: 100,
            status: 1,
          },
          session: {
            sid: 'geoip-home-fixture',
            current: true,
            login_method: 'password',
            ip: '127.0.0.1',
            user_agent: '',
            created_at: now,
            last_active_at: now,
            expires_at: now + 3600,
          },
        }
      } else if (pathname === '/api/setup') {
        data = { status: true }
      } else if (pathname === '/api/status') {
        data = { system_name: 'New API', setup: true }
      } else if (pathname === '/api/option/') {
        data = []
      }
      await route.fulfill({ json: { success: true, data } })
    })
    await authenticatedPage.goto(
      `${base}/system-settings/operations/status-images`
    )
    await authenticatedPage
      .getByLabel('图片站点名称', { exact: true })
      .waitFor()
    assert.equal(
      await authenticatedPage.getByRole('dialog', { name: /GeoIP/ }).count(),
      0
    )
    assert.equal(authenticatedLookups, 0)
    checks.push(
      'Authenticated settings page (API fixtures): no notice or GeoIP request'
    )
    assert.deepEqual(errors, [])
    console.log(JSON.stringify({ checks, errors }, null, 2))
  } finally {
    await browser.close()
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
