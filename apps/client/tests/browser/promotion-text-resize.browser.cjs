const { test } = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
const chrome = process.env.CLIENT_QA_CHROME_EXECUTABLE
const position = 'position-' + 'x'.repeat(119)
const id = '11111111-1111-1111-1111-111111111111'
const order = { id, channel: 'JD', maskedOrderId: '****1234', orderStatus: 'PAID', positionId: position, attributionMethod: 'SUB_ID', attributedAt: '2026-09-18T08:00:00Z', orderOccurredAt: '2026-09-18T08:00:00Z', statusAt: '2026-09-18T08:00:00Z' }
const counts = { timeZone: 'Asia/Shanghai', from: '2026-08-19T16:00:00Z', toExclusive: '2026-09-18T16:00:00Z', asOf: '2026-09-18T12:00:00.123456789Z', successfulLinks: 5, copyReports: 8, validOrders: 2 }

async function enlargedLayout(page) {
  const result = await page.evaluate(() => {
    window.qaFontSizes ||= new WeakMap()
    const sizes = [...document.querySelectorAll('uni-text,uni-button,uni-input input,.uni-input-placeholder')].map(element => [element, window.qaFontSizes.get(element) || parseFloat(getComputedStyle(element).fontSize)])
    for (const [element, size] of sizes) { window.qaFontSizes.set(element, size); element.style.fontSize = size * 2 + 'px' }
    const inputs = [...document.querySelectorAll('uni-input input')].map(element => ({ height: element.getBoundingClientRect().height, fontSize: parseFloat(getComputedStyle(element).fontSize) }))
    const targets = [...document.querySelectorAll('uni-button,[role=button]')].map(element => element.getBoundingClientRect()).filter(rect => rect.width > 0 && rect.height > 0)
    return { overflow: document.documentElement.scrollWidth > innerWidth, inputs, small: targets.some(rect => rect.width < 44 || rect.height < 44) }
  })
  assert.equal(result.overflow, false, 'enlarged long fields overflow the document')
  assert.equal(result.small, false, 'visible button target smaller than 44px')
  for (const input of result.inputs) assert.ok(input.height >= input.fontSize, `native input height ${input.height}px clips ${input.fontSize}px text`)
}

for (const kind of ['orders', 'dashboard']) test(`${kind} supports enlarged text with a provided test identity in nonempty, filter and recovery states`, async () => {
  const browser = await chromium.launch({ headless: true, ...(chrome ? { executablePath: chrome } : {}) })
  const errors = [], calls = []
  try {
    for (const width of [320, 390, 1280]) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      let mode = 'ready', selectedChannel = 'JD'
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.type() === 'error' && !/Failed to load resource.*(401|503)/.test(message.text())) errors.push(message.text()) })
      await page.route('**/api/v1/promoter/' + kind + '**', async route => {
        const request = route.request(), parsed = new URL(request.url()), query = parsed.searchParams
        calls.push({ url: parsed.pathname + parsed.search, token: request.headers().authorization, method: request.method() })
        const status = mode === 'error' ? 503 : mode === '401' ? 401 : 200
        let data
        if (kind === 'orders') {
          if (!parsed.pathname.endsWith(id)) selectedChannel = query.get('channel') || 'JD'
          const item = { ...order, channel: selectedChannel, positionId: query.get('positionId') || position }
          data = parsed.pathname.endsWith(id) ? { ...item, history: [{ previousStatus: 'CREATED', status: 'PAID', occurredAt: order.statusAt, projectedAt: order.statusAt }], refundEventCount: 2 } : { items: mode === 'zero' ? [] : [item], nextCursor: '' }
        } else data = { ...counts, ...(query.has('from') ? { from: query.get('from') + 'T00:00:00+08:00' } : {}), ...(query.has('to') ? { toExclusive: new Date(Date.parse(query.get('to') + 'T00:00:00+08:00') + 86400000).toISOString() } : {}), ...(mode === 'zero' ? { successfulLinks: 0, copyReports: 0, validOrders: 0 } : {}) }
        await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(status === 200 ? { code: 0, data } : { code: 'FAILED', message: 'private error' }) })
      })
      await page.goto(url + '/#/pages/promotion/' + kind)
      await page.getByText(/请登录后/).waitFor()
      const before = calls.length
      // Test-owned identity injection; no production fixture, token form or alternate page implementation.
      await page.evaluate(async kind => {
        const vue = await import('/node_modules/@dcloudio/uni-h5-vue/dist/vue.runtime.esm.js')
        const { default: Page } = await import('/src/pages/promotion/' + kind + '.vue')
        const { promoterSessionKey } = await import('/src/features/promotion/session.ts')
        const host = document.createElement('div'); document.body.replaceChildren(host)
        window.qaSession = vue.ref('qa-layout-identity')
        window.qaApp = vue.createApp(Page).provide(promoterSessionKey, window.qaSession); window.qaApp.mount(host)
      }, kind)
      const region = kind === 'orders' ? '[data-test=promoter-order]' : '[data-test=dashboard-counts]'
      await page.locator(region).waitFor(); assert.equal(calls.length, before + 1)
      await enlargedLayout(page)
      if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, `qa-promoter-${kind}-filter-text200-320.png`), fullPage: false })
      const prefix = kind === 'orders' ? 'order' : 'dashboard'
      const input = kind === 'orders' ? '[data-test=position-filter] input' : '[data-test=dashboard-position] input'
      await page.locator(input).fill(position)
      await page.locator(kind === 'orders' ? '[data-test=from-filter] input' : '[data-test=dashboard-from] input').fill('2026-09-18')
      await page.locator(kind === 'orders' ? '[data-test=to-filter] input' : '[data-test=dashboard-to] input').fill('2026-09-18')
      const channel = kind === 'orders' ? '[data-test=channel-TB]' : '[data-test=dashboard-channel-TB]'
      await page.locator(channel).focus(); await page.keyboard.press('Space')
      await page.locator(`[data-test=${prefix}-apply]`).focus(); await page.keyboard.press('Enter')
      await page.waitForLoadState('networkidle'); await page.locator(region).waitFor(); await enlargedLayout(page)
      const query = new URL(calls.at(-1).url, url).searchParams; assert.equal(query.get('positionId'), position); assert.equal(query.get('channel'), 'TB')
      assert.equal(query.get('from'), kind === 'orders' ? '2026-09-18T00:00:00.000Z' : '2026-09-18')
      assert.equal(query.get('to'), kind === 'orders' ? '2026-09-19T00:00:00.000Z' : '2026-09-18')
      if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) { await page.locator(input).evaluate(element => element.scrollIntoView({ block: 'center' })); await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, `qa-promoter-${kind}-input-text200-320.png`), fullPage: false }) }
      if (kind === 'orders') {
        mode = 'error'; await page.locator('[data-test=order-open]').focus(); await page.keyboard.press('Enter'); await page.getByText('重试订单详情', { exact: true }).waitFor(); await enlargedLayout(page)
        mode = 'ready'; await page.locator('[data-test=order-detail-retry]').focus(); await page.keyboard.press('Space'); await page.getByText('已创建 → 已付款', { exact: true }).waitFor(); await enlargedLayout(page)
        if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) { await page.locator('[data-test=order-detail]').scrollIntoViewIfNeeded(); await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, 'qa-promoter-orders-detail-text200-320.png'), fullPage: false }) }
        await page.locator('[data-test=order-close]').focus(); await page.keyboard.press('Enter')
      } else {
        mode = 'error'; await page.locator('[data-test=dashboard-refresh]').focus(); await page.keyboard.press('Enter'); await page.getByText(/暂不可用/).waitFor(); assert.equal(await page.locator(region).count(), 0); await enlargedLayout(page)
      }
      assert.ok(!(await page.locator('body').innerText()).includes('private error'))
      mode = 'ready'; await page.locator(`[data-test=${prefix}-refresh]`).focus(); await page.keyboard.press('Space'); await page.locator(region).waitFor(); await enlargedLayout(page)
      if (kind === 'dashboard' && process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) { await page.locator(region).evaluate(element => element.scrollIntoView({ block: 'start' })); await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, 'qa-promoter-dashboard-counts-text200-320.png'), fullPage: false }) }
      mode = 'zero'; await page.locator(`[data-test=${prefix}-refresh]`).click(); await page.getByText(kind === 'orders' ? /暂无本人推广订单/ : /本范围暂无统计记录/).waitFor(); await enlargedLayout(page)
      mode = '401'; await page.locator(`[data-test=${prefix}-refresh]`).click(); await page.getByText(/请登录后/).waitFor(); assert.equal(await page.locator(region).count(), 0)
      assert.equal(await page.locator('vite-error-overlay').count(), 0)
      await page.evaluate(() => window.qaApp.unmount()); await page.close()
    }
  } finally { await browser.close() }
  assert.deepEqual(errors, [])
  assert.ok(calls.every(call => call.method === 'GET' && call.token === 'Bearer qa-layout-identity' && !call.url.includes('identity')))
})
