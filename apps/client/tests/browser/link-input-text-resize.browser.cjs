const { test } = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { chromium } = require(process.env.CLIENT_QA_PLAYWRIGHT_MODULE || 'playwright')
const url = process.env.CLIENT_QA_URL || 'http://127.0.0.1:5173'
const content = 'https://item.jd.com/123.html ' + '测试长文案'.repeat(100)

async function layout(page) {
  const result = await page.evaluate(() => {
    window.qaFonts ||= new WeakMap()
    const nodes = document.querySelectorAll('uni-text,uni-button,uni-textarea textarea,.uni-textarea-placeholder,.input-label,.readiness-notice')
    const sizes = [...nodes].map(element => [element, window.qaFonts.get(element) || parseFloat(getComputedStyle(element).fontSize)])
    for (const [element, size] of sizes) { window.qaFonts.set(element, size); element.style.fontSize = size * 2 + 'px' }
    const input = document.querySelector('textarea'), rect = input.getBoundingClientRect()
    const targets = [...document.querySelectorAll('uni-button,[role=button]')].map(element => element.getBoundingClientRect()).filter(rect => rect.width && rect.height)
    return { overflow: document.documentElement.scrollWidth > innerWidth, small: targets.some(rect => rect.width < 44 || rect.height < 44), height: rect.height, width: rect.width, font: parseFloat(getComputedStyle(input).fontSize), name: input.getAttribute('aria-label') }
  })
  assert.equal(result.overflow, false)
  assert.equal(result.small, false)
  assert.ok(result.width >= 80 && result.height >= result.font, 'native textarea is clipped or unreadably narrow')
  assert.equal(result.name, '商品链接或文案')
}
async function key(page, action, value = 'Enter') { await page.locator(`[data-action=${action}]`).focus(); await page.keyboard.press(value) }

test('link preparation preserves long input, handles clipboard states and clears with enlarged text', async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.CLIENT_QA_CHROME_EXECUTABLE ? { executablePath: process.env.CLIENT_QA_CHROME_EXECUTABLE } : {}) })
  const errors = [], requests = []
  try {
    for (const width of [320, 390, 1280]) {
      const page = await browser.newPage({ viewport: { width, height: 844 } })
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
      page.on('request', request => { if (new URL(request.url()).pathname.startsWith('/api/')) requests.push(request.url()) })
      await page.goto(url + '/#/pages/promotion/convert'); await page.locator('textarea').waitFor()
      assert.equal(await page.title(), '准备推广链接')
      // Replace only the clipboard gateway in this disposable test tab, never OS clipboard.
      await page.evaluate(() => {
        window.qaClipboardMode = 'error'; window.qaClipboardCalls = 0
        window.uni.getClipboardData = callbacks => {
          window.qaClipboardCalls++
          if (window.qaClipboardMode === 'pending') window.qaClipboardResolve = value => callbacks.success({ data: value })
          else if (window.qaClipboardMode === 'error') callbacks.fail({ message: 'private clipboard error' })
          else callbacks.success({ data: window.qaClipboardMode === 'empty' ? '' : window.qaClipboardMode === 'oversize' ? '中'.repeat(1400) : 'https://item.jd.com/456.html' })
        }
      })
      assert.equal(await page.evaluate(() => window.qaClipboardCalls), 0)
      await page.locator('textarea').fill(content); await layout(page)
      for (const mode of ['error', 'empty', 'oversize']) {
        await page.evaluate(mode => window.qaClipboardMode = mode, mode); await key(page, 'paste', 'Space')
        await page.getByText(mode === 'error' ? /无法读取剪贴板/ : mode === 'empty' ? /剪贴板没有内容/ : /内容不能超过 4096/).waitFor()
        assert.equal(await page.locator('textarea').inputValue(), content); await layout(page)
      }
      assert.ok(!(await page.locator('body').innerText()).includes('private clipboard error'))
      await page.evaluate(() => window.qaClipboardMode = 'pending'); await key(page, 'paste')
      await page.getByText('正在读取…', { exact: true }).waitFor(); await layout(page)
      assert.equal(await page.locator('[data-action=clear]').getAttribute('aria-disabled'), 'true')
      assert.equal(await page.locator('textarea').isDisabled(), true)
      await page.locator('[data-action=clear]').dispatchEvent('keydown', { key: ' ' })
      assert.equal(await page.evaluate(() => window.qaClipboardCalls), 4)
      assert.equal(await page.locator('textarea').inputValue(), content)
      assert.equal(await page.locator('.link-input').getAttribute('aria-busy'), 'true')
      await page.evaluate(() => window.qaClipboardResolve('https://item.jd.com/456.html'))
      await page.getByText('已粘贴，请核对内容。', { exact: true }).waitFor(); await layout(page)
      await page.waitForFunction(() => document.querySelector('textarea').value === 'https://item.jd.com/456.html', null, { timeout: 5000 })
      assert.equal(await page.locator('textarea').inputValue(), 'https://item.jd.com/456.html')
      if (process.env.CLIENT_QA_SCREENSHOT_DIR && width === 320) await page.screenshot({ path: path.join(process.env.CLIENT_QA_SCREENSHOT_DIR, 'qa-link-input-text200-320.png'), fullPage: false })
      await page.locator('textarea').fill('old'); await page.locator('textarea').fill(content)
      await key(page, 'clear', 'Space'); await page.waitForLoadState('networkidle'); await layout(page)
      assert.equal(await page.locator('textarea').inputValue(), '')
      assert.equal(await page.locator('[data-action=preview]').getAttribute('aria-disabled'), 'true')
      assert.equal(await page.locator('vite-error-overlay').count(), 0)
      await page.close()
    }
  } finally { await browser.close() }
  assert.deepEqual(errors, []); assert.deepEqual(requests, [])
})
