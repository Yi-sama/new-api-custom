// Browser UI checks use API fixtures; Go tests separately verify database persistence.
/* eslint-disable @typescript-eslint/no-require-imports -- Resolve the external Playwright runtime through NODE_PATH without adding production dependencies. */
const { chromium } = require('playwright')
const sharp = require('sharp')
const fs = require('node:fs/promises')
const path = require('node:path')
const assert = require('node:assert/strict')

async function main() {
  const output = path.resolve(process.env.STATUS_IMAGE_OUTPUT || '../qq-status-image-check')
  await fs.mkdir(output, { recursive: true })
  let saved = null
  let rejectSave = false
  let liveFailure = false
  const browser = await chromium.launch({ channel: 'msedge', headless: true })
  let page
  try {
    const context = await browser.newContext({ locale: 'zh-CN', colorScheme: 'light', viewport: { width: 1440, height: 1000 } })
    page = await context.newPage()
    const errors = []
    page.on('pageerror', (error) => errors.push(error.message))
    page.on('console', (message) => { if (message.type() === 'error') console.error('Browser:', message.text()) })
    await page.route('**/cdn-cgi/trace', (route) => route.fulfill({ contentType: 'text/plain', body: 'loc=US\n' }))
    await page.route('**/api/**', async (route) => {
      const url = new URL(route.request().url())
      let data = []
      if (url.pathname === '/api/user/auth/refresh') {
        const now = Math.floor(Date.now() / 1000)
        data = {
          access_token: 'browser-test-only', token_type: 'Bearer', access_expires_at: now + 3600,
          user: { id: 1, username: 'preview', display_name: '本地预览', role: 100, status: 1 },
          session: { sid: 'preview-session', current: true, login_method: 'password', ip: '127.0.0.1', user_agent: '', created_at: now, last_active_at: now, expires_at: now + 3600 },
        }
      } else if (url.pathname === '/api/setup') {
        data = { status: true }
      } else if (url.pathname === '/api/status') {
        data = { system_name: 'New API', logo: '/logo.png', version: 'local-preview', setup: true }
      } else if (url.pathname === '/api/notice') {
        data = ''
      } else if (url.pathname === '/api/option/') {
        if (route.request().method() === 'PUT') {
          assert.equal(route.request().postDataJSON().key, 'StatusImageConfig')
          if (rejectSave) return route.fulfill({ json: { success: false, message: 'test save failure' } })
          saved = route.request().postDataJSON()
        }
        data = saved ? [saved] : []
      } else if (url.pathname === '/api/channel-status/overview') {
        if (liveFailure) return route.fulfill({ status: 503, json: { success: false, message: 'test offline' } })
        data = { enabled: true, mode: 'user', checked_at: 1, groups: [] }
      }
      await route.fulfill({ json: { success: true, message: '', data } })
    })
    const url = process.env.STATUS_IMAGE_URL || 'http://localhost:3000/system-settings/operations/status-images'
    await page.goto(url)
    const siteName = page.getByLabel('图片站点名称', { exact: true })
    await siteName.waitFor({ timeout: 20000 })
    await page.getByRole('button', { name: '下载 PNG' }).waitFor()
    await page.waitForFunction(() => document.querySelector('canvas[role=img]')?.getAttribute('aria-busy') === 'false')

    const widths = (process.env.STATUS_IMAGE_WIDTHS || '1440,390,320').split(',').map(Number)
    assert(widths.every((width) => Number.isInteger(width) && width >= 320))
    for (const width of widths) {
      await page.setViewportSize({ width, height: 1000 })
      const dimensions = await page.locator('section').filter({ has: siteName }).last().evaluate((section) => ({
        scroll: section.scrollWidth, client: section.clientWidth, right: section.getBoundingClientRect().right,
        overflow: [...section.querySelectorAll('*')].filter((element) => element.getBoundingClientRect().right > section.getBoundingClientRect().right + 1).slice(0, 8).map((element) => ({ tag: element.tagName, text: element.textContent?.slice(0, 60), class: element.className, right: element.getBoundingClientRect().right })),
      }))
      assert(dimensions.overflow.length === 0 && dimensions.right <= width + 1, `visible settings overflow at ${width}: ${JSON.stringify(dimensions)}`)
      const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth)
      if (documentWidth > width) {
        await page.goto(url.replace('/status-images', '/behavior'))
        await page.getByText('系统行为', { exact: true }).first().waitFor()
        assert((await page.evaluate(() => document.documentElement.scrollWidth)) > width, 'unexpected document overflow without a shared-layout baseline')
        console.log(`Baseline: shared app header also overflows at ${width}px; new settings section fits`)
        await page.goto(url)
        await siteName.waitFor()
      }
      await page.screenshot({ path: path.join(output, `settings-${width}.png`), fullPage: true })
    }
    await page.setViewportSize({ width: 1440, height: 1000 })

    for (const kind of ['channel', 'sub']) {
      await page.getByRole('tab', { name: kind === 'channel' ? '渠道状态' : 'Sub2API', exact: true }).click()
      await page.waitForFunction(() => document.querySelector('canvas[role=img]')?.getAttribute('aria-busy') === 'false')
      const download = page.waitForEvent('download')
      await page.getByRole('button', { name: '下载 PNG' }).click()
      const file = await download
      const target = path.join(output, `${kind}-anthropic.png`)
      await file.saveAs(target)
      const metadata = await sharp(target).metadata()
      const stats = await sharp(target).stats()
      assert.equal(metadata.width, 1200)
      assert(metadata.height > 500)
      assert(stats.channels.some((channel) => channel.stdev > 15), 'blank PNG')
      console.log(`${kind}: ${metadata.width} x ${metadata.height}, nonblank PNG`)
    }

    await siteName.fill('Yisama的自用站 · 本地测试')
    await page.getByRole('button', { name: '保存更改', exact: true }).click()
    await page.waitForFunction(() => [...document.querySelectorAll('button')].some((button) => button.textContent.includes('保存更改') && button.disabled))
    assert.equal(JSON.parse(saved.value).site_name, 'Yisama的自用站 · 本地测试')
    await page.reload()
    await siteName.waitFor()
    assert.equal(await siteName.inputValue(), 'Yisama的自用站 · 本地测试')
    await siteName.fill('')
    await page.getByRole('button', { name: '保存更改', exact: true }).click()
    assert.equal(await siteName.getAttribute('aria-invalid'), 'true')
    await siteName.fill('测试'.repeat(30))
    await page.getByLabel('图片页脚', { exact: true }).fill('页脚换行与长文本验证。'.repeat(12))
    await page.waitForFunction(() => document.querySelector('canvas[role=img]')?.getAttribute('aria-busy') === 'false')
    await page.screenshot({ path: path.join(output, 'long-text.png'), fullPage: true })
    rejectSave = true
    await page.getByRole('button', { name: '保存更改', exact: true }).click()
    await page.getByText('test save failure', { exact: true }).first().waitFor()
    assert.equal(await siteName.inputValue(), '测试'.repeat(30))
    await page.getByRole('button', { name: '放弃修改', exact: true }).click()

    await page.getByLabel('预览数据', { exact: true }).selectOption('live')
    await page.waitForFunction(() => document.querySelector('canvas[role=img]')?.getAttribute('aria-busy') === 'false')
    await page.screenshot({ path: path.join(output, 'empty-channel.png'), fullPage: true })
    liveFailure = true
    await page.getByRole('button', { name: '刷新', exact: true }).click()
    await page.getByText('读取渠道统计失败', { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: '下载 PNG' }).count(), 0)
    assert.deepEqual(errors, [])
    console.log(`PASS: responsive ${widths.join('/')}, PNG export, save/reload, validation, failed save, long text, empty/error states; API fixtures only`)
  } catch (error) {
    if (page) {
      console.error('URL:', page.url())
      console.error((await page.locator('body').innerText()).slice(0, 4500))
      await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true })
    }
    throw error
  } finally {
    await browser.close()
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1 })
