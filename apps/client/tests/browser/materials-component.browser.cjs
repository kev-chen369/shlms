const { test } = require('node:test')
const assert = require('node:assert/strict')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
test('real selection component reads fresh detail, refuses generation and clears platform context', async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE })
  const errors = [], calls = []
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
    const item = { id: '11111111-1111-1111-1111-111111111111', platform: 'JD', type: 'PRODUCT', title: '合成验收商品', endsAt: '2027-01-01T00:00:00Z', sourceUpdatedAt: '2026-09-19T00:00:00Z', ruleVersion: 'v1', region: { mode: 'NATIONWIDE' }, business: '', terminals: ['H5'] }
    await page.route('**/api/v1/promoter/materials**', async route => {
      const request = route.request(); calls.push({ url: request.url(), method: request.method(), authorization: request.headers().authorization })
      const detail = new URL(request.url()).pathname.endsWith(item.id)
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { capability: { allowed: true, reason: 'READY' }, ...(detail ? { item, availability: { available: true, reason: 'AVAILABLE' } } : { items: [item], nextCursor: '' }) } }) })
    })
    await page.goto(url + '/#/pages/promotion/index')
    await page.getByText('推广中心', { exact: true }).waitFor()
    // Test-owned host exercises the actual component/model/uni request, not a
    // production identity bypass or alternate UI implementation.
    await page.evaluate(async () => {
      const vue = await import('/node_modules/@dcloudio/uni-h5-vue/dist/vue.runtime.esm.js')
      const { default: Component } = await import('/src/components/PromotionMaterials.vue')
      const { createMaterialsModel } = await import('/src/features/promotion/materials-model.ts')
      const host = document.createElement('div'); host.style.padding = '16px'; document.body.replaceChildren(host)
      window.qaModel = createMaterialsModel(vue.ref('qa-material-identity'))
      window.qaApp = vue.createApp(Component, { model: window.qaModel }); window.qaApp.mount(host)
      window.qaModel.setScope({ platform: 'JD', type: 'PRODUCT', terminal: 'H5', positionId: 'p1', scene: 'group' })
    })
    await page.locator('[data-test=material-card]').waitFor()
    assert.equal(await page.locator('[data-test=material-generate]').getAttribute('disabled') !== null, true)
    await page.locator('[data-test=material-view]').focus(); await page.keyboard.press('Enter')
    await page.locator('[data-test=material-detail]').getByText('合成验收商品', { exact: true }).waitFor()
    assert.equal(calls.length, 2)
    assert.equal(await page.locator('vite-error-overlay').count(), 0)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
    if (process.env.CLIENT_QA_SCREENSHOT_DIR) await page.screenshot({ path: process.env.CLIENT_QA_SCREENSHOT_DIR + '/materials-component-390.png', fullPage: true })
    await page.locator('[data-test=material-detail-close]').focus(); await page.keyboard.press('Space')
    assert.equal(await page.locator('[data-test=material-detail]').count(), 0)
    await page.locator('[data-test=materials-platform-MEITUAN]').click()
    await page.getByText('请选择本人推广位与投放场景后读取目录。', { exact: true }).waitFor()
    assert.equal(await page.locator('[data-test=material-card]').count(), 0)
    assert.equal(calls.length, 2)
    assert.ok(calls.every(call => call.method === 'GET' && call.authorization === 'Bearer qa-material-identity'))
    await page.evaluate(() => { window.qaApp.unmount(); window.qaModel.dispose() })
    assert.deepEqual(errors, [])
  } finally { await browser.close() }
})
