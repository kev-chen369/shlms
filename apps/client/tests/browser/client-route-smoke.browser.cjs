const { test } = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
const pages = [
  ['home/index', '万宝单生活', '万宝单生活', '[data-test=search-submit]', /当前不会生成商品结果/],
  ['coupons/index', '万宝单生活', '领券中心', '[data-platform=PDD]', /尚未开放/],
  ['promotion/index', '万宝单生活', '推广中心', '[data-test=promoter-orders-entry]', /请登录后查看本人推广订单/],
  ['profile/index', '万宝单生活', '我的', '[data-test=promotion-secondary]', /推广业务尚未开放/],
  ['promotion/convert', '准备推广链接', '准备推广链接', '[data-action=clear]', /可以准备内容/],
  ['promotion/orders', '万宝单生活', '本人推广订单', '.entry-button', /推广业务尚未开放/],
  ['promotion/dashboard', '万宝单生活', '本人推广统计', '[data-test=dashboard-back]', /推广业务尚未开放/],
]

test('seven implemented routes retain default-state layout and keyboard navigation at three widths', async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const errors = [], privateRequests = []
  try {
    for (const width of [320, 390, 1280]) for (const [route, title, heading, control, expected] of pages) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
      page.on('request', request => { if (new URL(request.url()).pathname.startsWith('/api/v1/promoter/')) privateRequests.push(request.url()) })
      await page.route('**/api/v1/coupon**', request => request.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { items: [], ...(request.request().url().includes('/coupons?') ? { nextCursor: '' } : {}) } }) }))
      await page.goto(url + '/#/pages/' + route); await page.locator('.client-page').waitFor(); await page.waitForLoadState('networkidle')
      assert.equal(await page.title(), title)
      assert.ok((await page.locator('.client-page').innerText()).includes(heading))
      assert.equal(await page.locator('vite-error-overlay').count(), 0)
      for (const scale of [1, 2]) {
        if (scale === 2) await page.evaluate(() => {
          const sizes = [...document.querySelectorAll('uni-text,uni-button,uni-input input,textarea,.uni-input-placeholder,.uni-textarea-placeholder,.uni-tabbar__label,.input-label,.readiness-notice,.context-caption')].map(element => [element, parseFloat(getComputedStyle(element).fontSize)])
          for (const [element, size] of sizes) element.style.fontSize = size * 2 + 'px'
        })
        const result = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth > innerWidth, small: [...document.querySelectorAll('uni-button,[role=button]')].map(element => element.getBoundingClientRect()).filter(rect => rect.width && rect.height).some(rect => rect.width < 44 || rect.height < 44) }))
        assert.equal(result.overflow, false, route + ' overflow at ' + width + '/' + scale)
        assert.equal(result.small, false, route + ' small target at ' + width + '/' + scale)
      }
      if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320 && ['profile/index', 'promotion/index'].includes(route)) await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, `qa-${route.split('/')[0]}-route-text200-320.png`), fullPage: false })
      await page.locator(control).focus(); await page.keyboard.press('Space'); await page.getByText(expected).first().waitFor()
      if (route === 'promotion/convert') assert.equal(await page.locator('textarea').inputValue(), '')
      await page.close()
    }
  } finally { await browser.close() }
  assert.deepEqual(errors, []); assert.deepEqual(privateRequests, [])
})
