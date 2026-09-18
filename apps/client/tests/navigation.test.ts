import { afterEach, expect, it } from 'vitest'
import { enhanceH5Navigation } from '../src/platform/h5-navigation'

afterEach(() => document.body.replaceChildren())
it('makes framework tab items focusable and invokes the existing click once for Enter or Space', async () => {
  const bar = document.createElement('div')
  document.body.append(bar)
  const dispose = enhanceH5Navigation(document.body)
  bar.innerHTML = '<div class="uni-tabbar__item"><span class="uni-tabbar__label">分类</span></div>'
  await new Promise(resolve => setTimeout(resolve, 0))
  const item = bar.firstElementChild as HTMLElement
  let activations = 0
  item.addEventListener('click', () => activations++)
  expect(item.tabIndex).toBe(0)
  expect(item.getAttribute('aria-label')).toBe('分类')
  item.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  expect(activations).toBe(1)
  const space = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true })
  item.dispatchEvent(space)
  expect(activations).toBe(2)
  expect(space.defaultPrevented).toBe(true)
  item.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
  expect(activations).toBe(2)
  dispose()
  item.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  expect(activations).toBe(2)
})
