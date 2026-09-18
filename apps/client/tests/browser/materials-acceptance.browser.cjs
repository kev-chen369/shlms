const { test } = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
const id = '11111111-1111-1111-1111-111111111111'
const nextId = '22222222-2222-2222-2222-222222222222'
const position = { id: 'p-' + 'x'.repeat(126), name: '合成本人推广位' + '长'.repeat(60), scene: 'scene-' + 'x'.repeat(74), status: 'ENABLED', isDefault: true, canConvert: false, channels: [{ channel: 'JD', readiness: 'WAITING_VERIFICATION' }] }
const item = { id, platform: 'JD', type: 'PRODUCT', title: '合成商品' + '长'.repeat(60), endsAt: '2027-01-01T00:00:00Z', sourceUpdatedAt: '2026-09-19T00:00:00.123456789Z', ruleVersion: 'rule-' + 'x'.repeat(75), region: { mode: 'CITIES', cityCodes: ['310100'] }, business: 'food', terminals: ['H5', 'WX_MINI'] }
const capability = { allowed: true, reason: 'READY' }
async function activate(page, selector, key = 'Enter') { await page.locator(selector).first().focus(); await page.keyboard.press(key) }
async function mounted(page) {
  await page.evaluate(async () => {
    const vue = await import('/node_modules/@dcloudio/uni-h5-vue/dist/vue.runtime.esm.js')
    const { default: Page } = await import('/src/pages/promotion/materials.vue')
    const { promoterSessionKey } = await import('/src/features/promotion/session.ts')
    const host = document.createElement('div'); document.body.replaceChildren(host)
    window.qaSession = vue.ref('qa-material-acceptance')
    window.qaApp = vue.createApp(Page); window.qaApp.provide(promoterSessionKey, window.qaSession); window.qaApp.mount(host)
  })
}
async function layout(page, enlarge) {
  const result = await page.evaluate(enlarge => {
    window.qaFontSizes ||= new WeakMap()
    if (enlarge) {
      const sizes = [...document.querySelectorAll('uni-text,uni-button,uni-input input,.uni-input-placeholder')].map(element => [element, window.qaFontSizes.get(element) || parseFloat(getComputedStyle(element).fontSize)])
      for (const [element, size] of sizes) { window.qaFontSizes.set(element, size); element.style.fontSize = size * 2 + 'px' }
    }
    const targets = [...document.querySelectorAll('uni-button,[role=button]')].map(element => element.getBoundingClientRect()).filter(rect => rect.width && rect.height)
    const inputs = [...document.querySelectorAll('uni-input input')].map(element => ({ height: element.getBoundingClientRect().height, font: parseFloat(getComputedStyle(element).fontSize) }))
    return { overflow: document.documentElement.scrollWidth > innerWidth, small: targets.some(rect => rect.width < 44 || rect.height < 44), inputs }
  }, enlarge)
  assert.equal(result.overflow, false, 'document overflow')
  assert.equal(result.small, false, 'button target smaller than 44px')
  for (const input of result.inputs) assert.ok(input.height >= input.font, 'native input clips enlarged text')
  assert.equal(await page.locator('vite-error-overlay').count(), 0)
}
async function shot(page, filename, selector) {
  if (!process.env.CLIENT_QA_SCREENSHOT_DIR) return
  if (selector) await page.locator(selector).first().evaluate(element => element.scrollIntoView({ block: 'start' }))
  else await page.evaluate(() => window.scrollTo(0, 0))
  await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, filename), fullPage: false })
}
for (const width of [320, 390, 1280]) test(`material selection at ${width}px supports 200% text, keyboard and recoverable actual page states`, async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const calls = [], errors = []
  try {
    const page = await browser.newPage({ viewport: { width, height: 844 } })
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.type() === 'error' && !/Failed to load resource.*(401|503)/.test(message.text())) errors.push(message.text()) })
    let positionMode = 'ready', materialMode = 'ready', detailMode = 'ready'
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), parsed = new URL(request.url()), q = parsed.searchParams
      calls.push({ url: parsed.pathname + parsed.search, method: request.method(), token: request.headers().authorization })
      const positions = parsed.pathname === '/api/v1/promotion-positions', detail = parsed.pathname.endsWith(id)
      const mode = positions ? positionMode : detail ? detailMode : materialMode
      const status = mode === 'error' ? 503 : mode === '401' ? 401 : 200
      const type = q.get('type') || 'PRODUCT'
      const current = { ...item, type, ...(type === 'ACTIVITY' ? { title: '合成授权活动', startsAt: '2026-09-01T00:00:00Z' } : {}) }
      let data
      if (positions) data = { items: mode === 'empty' ? [] : [q.has('cursor') ? { ...position, id: 'p2', name: '合成第二本人位' } : position], nextCursor: mode === 'ready' && !q.has('cursor') ? 'cGFnZTI' : '' }
      else if (detail) data = mode === 'expired' ? { capability, availability: { available: false, reason: 'EXPIRED' } } : { capability, availability: { available: true, reason: 'AVAILABLE' }, item: { ...current, title: type === 'ACTIVITY' ? '复核活动' : '复核商品' } }
      else data = mode === 'blocked' ? { items: [], capability: { allowed: false, reason: 'SUSPENDED' } } : { capability, items: mode === 'empty' ? [] : [q.has('cursor') ? { ...current, id: nextId, title: '合成续页商品' } : current], nextCursor: mode === 'ready' && !q.has('cursor') ? 'bWF0ZXJpYWwy' : '' }
      await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(status === 200 ? { code: 0, data } : { code: 'FAILED', message: 'private-upstream-secret' }) })
    })
    await page.goto(url + '/#/pages/promotion/index')
    await activate(page, '[data-test=promoter-materials-entry]', 'Space')
    await page.getByText(/请登录后查看本人推广选品/).waitFor()
    assert.ok(page.url().includes('/pages/promotion/materials')); assert.equal(await page.title(), '万宝单生活'); assert.equal(calls.length, 0)
    await layout(page, false)
    if (width === 390) await shot(page, 'qa-materials-signed-out-390.png')
    await mounted(page); await page.locator('[data-test=position-choice]').waitFor()
    await layout(page, false); await layout(page, true)
    assert.equal(await page.locator('[data-test=position-confirm]').count(), 0)
    positionMode = 'error'; await activate(page, '[data-test=positions-more]')
    await page.getByText('推广位读取失败，请稍后重试', { exact: true }).waitFor()
    assert.equal(await page.locator('[data-test=position-choice]').count(), 1); await layout(page, true)
    positionMode = 'ready'; await activate(page, '[data-test=positions-more]', 'Space')
    await page.waitForFunction(() => document.querySelectorAll('[data-test=position-choice]').length === 2)
    await layout(page, true)
    await activate(page, '[data-test=position-choice]:first-of-type')
    await page.locator('[data-test=position-confirm]').waitFor()
    await page.locator('uni-input[aria-label="城市编码"] input').fill('310100')
    await page.locator('uni-input[aria-label="业务范围"] input').fill('food')
    await layout(page, true)
    if (width === 320) await shot(page, 'qa-materials-selection-text200-320.png', '.positions')
    await activate(page, '[data-test=position-confirm]', 'Space')
    await page.locator('[data-test=material-card]').waitFor(); await layout(page, true)
    const query = new URL(calls.at(-1).url, url).searchParams
    assert.equal(query.get('positionId'), position.id); assert.equal(query.get('scene'), position.scene)
    assert.equal(query.get('cityCode'), '310100'); assert.equal(query.get('business'), 'food'); assert.equal(query.get('terminal'), 'H5')
    assert.equal(await page.locator('[data-test=material-generate]').getAttribute('disabled') !== null, true)
    if (width === 320) await shot(page, 'qa-materials-product-text200-320.png', '[data-test=material-card]')
    if (width === 1280) await shot(page, 'qa-materials-desktop-text200-1280.png', '[data-test=material-card]')
    materialMode = 'error'; await activate(page, '[data-test=materials-more]')
    await page.getByText('推广物料读取失败，请稍后重试', { exact: true }).waitFor()
    assert.equal(await page.locator('[data-test=material-card]').count(), 1); await layout(page, true)
    materialMode = 'ready'; await activate(page, '[data-test=materials-more]', 'Space')
    await page.waitForFunction(() => document.querySelectorAll('[data-test=material-card]').length === 2)
    assert.equal(await page.locator('[data-test=materials-more]').count(), 0); await layout(page, true)
    detailMode = 'error'; await activate(page, '[data-test=material-view]:first-of-type')
    await page.locator('[data-test=material-detail-retry]').waitFor(); await layout(page, true)
    detailMode = 'ready'; await activate(page, '[data-test=material-detail-retry]', 'Space')
    await page.locator('[data-test=material-detail]').getByText('复核商品', { exact: true }).waitFor(); await layout(page, true)
    if (width === 320) await shot(page, 'qa-materials-detail-text200-320.png', '[data-test=material-detail]')
    await activate(page, '[data-test=material-detail-close]')
    materialMode = 'empty'; await activate(page, '[data-test=materials-refresh]')
    await page.getByText(/本范围暂无适用物料/).waitFor(); assert.equal(await page.locator('[data-test=material-card]').count(), 0); await layout(page, true)
    materialMode = 'blocked'; await activate(page, '[data-test=materials-refresh]', 'Space')
    await page.getByText(/目录已停用/).waitFor(); await layout(page, true)
    if (width === 320) await shot(page, 'qa-materials-blocked-text200-320.png', '[data-test=materials-refresh]')
    materialMode = 'ready'; await activate(page, '[data-test=materials-refresh]'); await page.locator('[data-test=material-card]').waitFor()
    detailMode = 'expired'; await activate(page, '[data-test=material-view]')
    await page.getByText(/该物料已失效或不适用/).waitFor(); assert.equal(await page.locator('[data-test=material-card]').count(), 0); await layout(page, true)
    await activate(page, '[data-test=material-detail-close]')
    await activate(page, '[data-test=materials-type-ACTIVITY]', 'Space')
    assert.equal(await page.locator('[data-test=position-choice]').count(), 0); assert.equal(await page.locator('[data-test=position-confirm]').count(), 0)
    await activate(page, '[data-test=positions-refresh]'); await page.locator('[data-test=position-choice]').waitFor()
    await activate(page, '[data-test=position-choice]')
    assert.equal(await page.locator('uni-input[aria-label="城市编码"] input').inputValue(), '')
    await page.locator('uni-input[aria-label="城市编码"] input').fill('310100')
    await page.locator('uni-input[aria-label="业务范围"] input').fill('food')
    await activate(page, '[data-test=position-confirm]'); await page.getByText('合成授权活动', { exact: true }).waitFor(); await layout(page, true)
    assert.ok((await page.locator('[data-test=material-card]').innerText()).includes('开始：'))
    assert.ok(!(await page.locator('[data-test=material-card]').innerText()).includes('预计收益'))
    if (width === 320) await shot(page, 'qa-materials-activity-text200-320.png', '[data-test=material-card]')
    const before = calls.length
    await activate(page, '[data-test=materials-platform-TAOBAO]', 'Space'); await page.getByText(/当前接口没有淘宝推广位映射/).waitFor()
    await activate(page, '[data-test=materials-platform-MEITUAN]'); await page.getByText(/当前接口没有美团推广位映射/).waitFor()
    await page.locator('[data-test=materials-type-PRODUCT]').evaluate(element => element.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })))
    assert.equal(await page.locator('[data-test=materials-type-ACTIVITY]').getAttribute('aria-pressed'), 'true')
    assert.equal(calls.length, before); await layout(page, true)
    await page.evaluate(() => { window.qaSession.value = null }); await page.getByText(/请登录后查看本人推广选品/).waitFor()
    assert.equal(await page.locator('.positions').count(), 0); await layout(page, true)
    assert.ok(!(await page.locator('body').innerText()).includes('private-upstream-secret'))
    await page.evaluate(() => window.qaApp.unmount())
  } finally { await browser.close() }
  assert.deepEqual(errors, []); assert.ok(calls.every(call => call.method === 'GET' && call.token === 'Bearer qa-material-acceptance'))
})

test('actual material page retires late responses and identity epochs without ignoring current identity rejection', { timeout: 20000 }, async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const calls = [], errors = []
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', message => { if (message.type() === 'error' && !/Failed to load resource.*401/.test(message.text())) errors.push(message.text()) })
    let holdKind = '', heldResolve, release, heldRequest
    function hold(kind) { holdKind = kind; return new Promise(resolve => { heldResolve = resolve }) }
    const safeItem = { ...item, title: '合成竞态目录', region: { mode: 'NATIONWIDE', cityCodes: [] }, business: '' }
    await page.route('**/api/v1/**', async route => {
      const request = route.request(), pathname = new URL(request.url()).pathname
      const kind = pathname === '/api/v1/promotion-positions' ? 'positions' : pathname.endsWith(id) ? 'detail' : 'list'
      calls.push({ kind, method: request.method(), token: request.headers().authorization })
      const data = kind === 'positions' ? { items: [position] } : kind === 'detail' ? { capability, availability: { available: true, reason: 'AVAILABLE' }, item: safeItem } : { capability, items: [safeItem] }
      let status = 200
      if (kind === holdKind) { holdKind = ''; heldRequest = request; const pending = new Promise(resolve => { release = resolve }); heldResolve(); status = await pending }
      await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(status === 200 ? { code: 0, data } : { code: 'FAILED', message: 'private-upstream-secret' }) })
    })
    async function readyPosition() { await activate(page, '[data-test=positions-refresh]'); await page.locator('[data-test=position-choice]').waitFor(); await activate(page, '[data-test=position-choice]') }
    async function retire(status = 200) {
      const response = page.waitForResponse(response => response.request() === heldRequest)
      release(status); assert.equal(await (await response).finished(), null)
      await page.waitForLoadState('networkidle')
    }
    await page.goto(url + '/#/pages/promotion/materials'); await mounted(page)
    await page.locator('[data-test=position-choice]').waitFor(); await activate(page, '[data-test=position-choice]')
    let pending = hold('list'); await activate(page, '[data-test=position-confirm]'); await pending
    await activate(page, '[data-test=materials-platform-TAOBAO]'); await retire()
    assert.equal(await page.locator('[data-test=material-card]').count(), 0)
    assert.equal(await page.locator('[data-test=position-confirm]').count(), 0)
    await activate(page, '[data-test=materials-platform-JD]'); await readyPosition()
    await activate(page, '[data-test=position-confirm]'); await page.locator('[data-test=material-card]').waitFor()
    pending = hold('detail'); await activate(page, '[data-test=material-view]'); await pending
    await activate(page, '[data-test=materials-platform-MEITUAN]'); await retire()
    assert.equal(await page.locator('[data-test=material-detail]').count(), 0)
    assert.equal(await page.locator('[data-test=material-card]').count(), 0)
    await activate(page, '[data-test=materials-platform-JD]')
    pending = hold('positions'); await activate(page, '[data-test=positions-refresh]'); await pending
    await page.evaluate(() => { window.qaSession.value = 'qa-identity-B' }); await readyPosition(); await retire(401)
    assert.equal(await page.locator('[data-test=position-confirm]').count(), 1, 'old identity 401 must not reject new identity')
    pending = hold('positions'); await activate(page, '[data-test=positions-refresh]'); await pending
    await activate(page, '[data-test=materials-type-ACTIVITY]'); await readyPosition(); await retire(401)
    await page.getByText(/请登录后查看本人推广选品/).waitFor()
    assert.equal(await page.locator('[data-test=position-choice]').count(), 0, 'same identity 401 clears even superseded view')
    await page.evaluate(() => { window.qaSession.value = 'qa-identity-C' })
    await activate(page, '[data-test=materials-type-PRODUCT]'); await readyPosition()
    pending = hold('list'); await activate(page, '[data-test=position-confirm]'); await pending
    const before = calls.length
    await page.evaluate(async () => { window.qaSession.value = 'qa-identity-D'; await Promise.resolve(); window.qaSession.value = 'qa-identity-C' })
    await retire(); assert.equal(calls.length, before, 'identity changes never auto-read')
    assert.equal(await page.locator('[data-test=material-card]').count(), 0, 'ABA identity cannot revive old catalog')
    assert.equal(await page.locator('[data-test=position-confirm]').count(), 0)
    await readyPosition(); await page.locator('uni-input[aria-label="城市编码"] input').fill('310100')
    await page.evaluate(() => { window.qaSession.value = 'qa-identity-E' }); await readyPosition()
    assert.equal(await page.locator('uni-input[aria-label="城市编码"] input').inputValue(), '', 'native draft must not cross identity')
    assert.ok(!(await page.locator('body').innerText()).includes('private-upstream-secret'))
    await page.evaluate(() => window.qaApp.unmount())
  } finally { await browser.close() }
  assert.deepEqual(errors, []); assert.ok(calls.every(call => call.method === 'GET' && /^Bearer qa-/.test(call.token)))
})
