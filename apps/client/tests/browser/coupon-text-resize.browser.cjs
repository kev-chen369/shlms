const { test } = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
const business = 'business-' + 'x'.repeat(31)
const updatedAt = '2026-09-18T00:00:00Z', expiresAt = '2032-01-01T00:00:00Z'

async function layout(page) {
  const result = await page.evaluate(() => {
    window.qaFonts ||= new WeakMap()
    const sizes = [...document.querySelectorAll('uni-text,uni-button,uni-input input,.uni-input-placeholder,.context-caption')].map(element => [element, window.qaFonts.get(element) || parseFloat(getComputedStyle(element).fontSize)])
    for (const [element, size] of sizes) { window.qaFonts.set(element, size); element.style.fontSize = size * 2 + 'px' }
    const targets = [...document.querySelectorAll('uni-button,[role=button]')].map(element => element.getBoundingClientRect()).filter(rect => rect.width && rect.height)
    const input = document.querySelector('[data-test=coupon-business] input')
    return { overflow: document.documentElement.scrollWidth > innerWidth, small: targets.some(rect => rect.width < 44 || rect.height < 44), inputHeight: input.getBoundingClientRect().height, font: parseFloat(getComputedStyle(input).fontSize) }
  })
  assert.equal(result.overflow, false, 'coupon/product text overflows viewport')
  assert.equal(result.small, false, 'visible button target below 44px')
  assert.ok(result.inputHeight >= result.font, `business input height ${result.inputHeight}px clips ${result.font}px text`)
}
async function key(page, selector, value = 'Enter') { await page.locator(selector).focus(); await page.keyboard.press(value) }

test('nonempty coupons and products support enlarged text, keyboard filters and explicit pagination recovery', async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const errors = [], calls = []
  try {
    for (const width of [320, 390, 1280]) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      let productMode = 'ready', pageError = false
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.type() === 'error' && !/Failed to load resource.*503/.test(message.text())) errors.push(message.text()) })
      await page.route('**/api/v1/coupon**', async route => {
        const request = route.request(), parsed = new URL(request.url()), query = parsed.searchParams
        calls.push({ path: parsed.pathname, query: parsed.search, method: request.method(), authorization: request.headers().authorization })
        const products = parsed.pathname.endsWith('/products'), more = query.has('cursor')
        const failed = (more && pageError) || (products && productMode === 'error')
        const coupon = id => ({ id, platform: 'JD', claimMode: 'BUNDLED_OFFER', actionLabel: '领券购买', title: '测试只读商品券-' + 'long'.repeat(40), scope: 'PRODUCT', scopeExternalId: '', scopeName: '测试适用范围-' + 'scope'.repeat(20), currency: 'CNY', discountMinor: 500, thresholdMinor: 2000, cityCode: query.get('cityCode') || '', cityName: query.has('cityCode') ? '测试城市名称' : '', business: query.get('business') || '', ruleVersion: 'v-' + 'r'.repeat(78), updatedAt, expiresAt })
        let data
        if (parsed.pathname.endsWith('coupon-cities')) data = { items: [{ code: '110100', name: '测试城市名称' }] }
        else if (products) data = { items: productMode === 'empty' ? [] : [{ externalProductId: 'sku-' + (more ? '2' : '1') + 'x'.repeat(100), title: '测试适用商品-' + 'long'.repeat(40), updatedAt, expiresAt }], nextCursor: productMode === 'empty' || more ? '' : 'product-next' }
        else if (parsed.pathname.endsWith('/coupons')) data = { items: [coupon(more ? 'coupon-2' : 'coupon-1')], nextCursor: more ? '' : 'coupon-next' }
        else data = coupon('coupon-1')
        await route.fulfill({ status: failed ? 503 : 200, contentType: 'application/json', body: JSON.stringify(failed ? { code: 'FAILED', message: 'private error' } : { code: 0, data }) })
      })
      await page.goto(url + '/#/pages/coupons/index')
      assert.equal(await page.title(), '万宝单生活')
      await page.locator('[data-test=coupon-card]').waitFor(); await layout(page)
      if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, 'qa-coupon-list-text200-320.png'), fullPage: false })
      await key(page, '[data-test=coupon-city-toggle]')
      await key(page, '[data-city="110100"]', 'Space')
      await page.waitForLoadState('networkidle')
      assert.equal(new URLSearchParams(calls.at(-1).query).get('cityCode'), '110100')
      await page.locator('[data-test=coupon-business] input').fill('old')
      await page.locator('[data-test=coupon-business] input').fill(business)
      await key(page, '[data-test=coupon-business-apply]')
      await page.waitForLoadState('networkidle')
      assert.equal(new URLSearchParams(calls.at(-1).query).get('business'), business, 'apply must use the latest native business value')
      await layout(page)
      pageError = true; await key(page, '[data-test=coupon-more]'); await page.getByText(/可重试加载更多/).waitFor()
      assert.equal(await page.locator('[data-test=coupon-card]').count(), 1); await layout(page)
      pageError = false; await key(page, '[data-test=coupon-more]', 'Space'); await page.waitForLoadState('networkidle')
      await page.locator('[data-test=coupon-card]').nth(1).waitFor()
      assert.equal(await page.locator('[data-test=coupon-card]').count(), 2)
      await page.locator('[data-test=coupon-detail-open]').first().focus(); await page.keyboard.press('Enter')
      await page.locator('[data-test=coupon-product]').waitFor(); await layout(page)
      pageError = true; await key(page, '[data-test=coupon-products-more]'); await page.getByText('适用商品加载失败，可重试加载更多。', { exact: true }).waitFor()
      assert.equal(await page.locator('[data-test=coupon-product]').count(), 1); await layout(page)
      pageError = false; await key(page, '[data-test=coupon-products-more]', 'Space'); await page.waitForLoadState('networkidle')
      await page.locator('[data-test=coupon-product]').nth(1).waitFor()
      assert.equal(await page.locator('[data-test=coupon-product]').count(), 2); await layout(page)
      if (process.env.CLIENT_QA_SCREENSHOT_DIR) { await page.locator('[data-test=coupon-products]').evaluate(element => element.scrollIntoView({ block: 'start' })); await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, `qa-coupon-products-text200-${width}.png`), fullPage: false }) }
      for (const mode of ['error', 'empty', 'ready']) {
        productMode = mode; await key(page, '[data-test=coupon-products-retry]', 'Space'); await page.waitForLoadState('networkidle'); await layout(page)
        if (mode === 'ready') await page.locator('[data-test=coupon-product]').waitFor()
        else await page.locator('[data-test=coupon-products-status]').filter({ hasText: mode === 'error' ? /读取失败/ : /暂无/ }).waitFor()
        await layout(page)
        assert.equal(await page.locator('[data-test=coupon-product]').count(), mode === 'ready' ? 1 : 0)
        assert.equal(await page.locator('[data-test=coupon-detail]').count(), 1)
      }
      assert.ok(!(await page.locator('body').innerText()).includes('private error'))
      assert.equal(await page.locator('[data-test=coupon-products] a').count(), 0)
      await page.getByText(/领取与购买暂未接通，不表示券已领取/).waitFor()
      await key(page, '[data-test=coupon-detail-close]'); assert.equal(await page.locator('[data-test=coupon-products]').count(), 0)
      await page.locator('[data-test=coupon-detail-open]').first().focus(); await page.keyboard.press('Enter'); await page.locator('[data-test=coupon-product]').waitFor()
      await key(page, '[data-platform=PDD]', 'Space'); await page.locator('[data-test=coupon-status]').filter({ hasText: /尚未开放/ }).waitFor()
      assert.equal(await page.locator('[data-test=coupon-card]').count(), 0)
      assert.equal(await page.locator('[data-test=coupon-products]').count(), 0)
      assert.equal(await page.locator('vite-error-overlay').count(), 0)
      await page.close()
    }
  } finally { await browser.close() }
  assert.deepEqual(errors, [])
  assert.ok(calls.every(call => call.method === 'GET' && !call.authorization))
})
