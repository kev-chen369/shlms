// Run with Node, not Vitest/jsdom: this regression depends on real layout.
// Supply an existing Playwright module and Chrome executable if not installed locally.
const { test } = require('node:test')
const assert = require('node:assert/strict')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'

test('homepage keeps a readable search input with 200% user-style text at mobile and desktop widths', async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const errors = []
  try {
    for (const width of [320, 390, 1280]) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
      await page.route('**/api/v1/coupon-cities**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { items: [] } }) }))
      await page.route('**/api/v1/coupons**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { items: [], nextCursor: '' } }) }))
      await page.goto(url + '/#/pages/home/index')
      await page.locator('.brand').waitFor()
      await page.waitForLoadState('networkidle')
      assert.equal(await page.title(), '万宝单生活')
      for (const scale of [1, 2]) {
        if (scale === 2) await page.evaluate(() => {
          const sizes = [...document.querySelectorAll('uni-text,uni-button,uni-input input,.uni-input-placeholder,.uni-tabbar__label')].map(element => [element, parseFloat(getComputedStyle(element).fontSize)])
          for (const [element, size] of sizes) element.style.fontSize = size * 2 + 'px'
        })
        const input = await page.locator('.search-bar input').boundingBox()
        const fontSize = await page.locator('.search-bar input').evaluate(element => parseFloat(getComputedStyle(element).fontSize))
        assert.ok(input.width >= 80, `${width}px at ${scale * 100}%: search input ${input.width}px is unreadably narrow`)
        assert.ok(input.height >= fontSize, `${width}px at ${scale * 100}%: input height ${input.height}px clips ${fontSize}px text`)
        if (width === 390 && scale === 1) {
          const brand = await page.locator('.brand').boundingBox()
          assert.ok(brand.y < input.y + input.height && brand.y + brand.height > input.y, 'default 390px layout should keep brand and search on the same row')
        }
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${width}px at ${scale * 100}% overflow`)
      }
      await page.locator('[data-test=search-submit]').focus()
      await page.keyboard.press('Enter')
      await page.getByText(/当前不会生成商品结果/).waitFor()
      assert.equal(await page.locator('vite-error-overlay').count(), 0)
      await page.close()
    }
  } finally { await browser.close() }
  assert.deepEqual(errors, [])
})
