const { test } = require('node:test')
const assert = require('node:assert/strict')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'

test('rapid native business edits apply the latest value and platform resets retire pending input', async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  try {
    for (const width of [320, 390, 1280]) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      const queries = [], errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.route('**/api/v1/coupon**', async route => {
        const parsed = new URL(route.request().url())
        if (parsed.pathname.endsWith('/coupons')) queries.push(parsed.searchParams)
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { items: [], ...(parsed.pathname.endsWith('/coupons') ? { nextCursor: '' } : {}) } }) })
      })
      await page.goto(url + '/#/pages/coupons/index'); await page.locator('[data-test=coupon-status]').filter({ hasText: '暂无可用优惠' }).waitFor()
      const input = page.locator('[data-test=coupon-business] input')
      for (const method of ['button', 'input']) {
        await input.fill('old'); await input.fill('latest-' + method)
        if (method === 'button') await page.locator('[data-test=coupon-business-apply]').focus()
        await page.keyboard.press('Enter')
        await page.waitForFunction(expected => document.querySelector('.context-caption').textContent.includes(expected), 'latest-' + method)
        await page.locator('[data-test=coupon-status]').filter({ hasText: '暂无可用优惠' }).waitFor()
        assert.equal(queries.at(-1).get('business'), 'latest-' + method)
      }
      await input.fill('pending-old'); await input.fill('pending-new')
      await page.locator('[data-platform=TAOBAO]').focus(); await page.keyboard.press('Space')
      await page.waitForFunction(() => document.querySelector('.context-caption').textContent.includes('不限业务活动'))
      await page.waitForLoadState('networkidle')
      assert.equal(await input.inputValue(), '')
      await page.locator('[data-test=coupon-business-apply]').focus(); await page.keyboard.press('Enter')
      assert.equal(queries.at(-1).get('business'), null)
      assert.equal(queries.at(-1).get('platform'), 'TB')
      assert.deepEqual(errors, []); await page.close()
    }
  } finally { await browser.close() }
})
