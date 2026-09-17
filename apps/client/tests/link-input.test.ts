import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import LinkInput from '../src/components/LinkInput.vue'

describe('local product link input', () => {
  it('rejects input beyond the server 4096-byte UTF-8 limit without replacing original content', async () => {
    const wrapper = mount(LinkInput, { props: { ready: true, readClipboard: async () => '中'.repeat(1366) } })
    await wrapper.get('textarea').setValue('原输入')
    await wrapper.get('[data-action=paste]').trigger('click')
    await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('原输入')
    expect(wrapper.text()).toContain('4096')
  })
  it('supports keyboard paste and clear as well as click', async () => {
    const wrapper = mount(LinkInput, { props: { ready: false, readClipboard: async () => '商品链接' } })
    await wrapper.get('[data-action=paste]').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('商品链接')
    await wrapper.get('[data-action=clear]').trigger('keydown', { key: ' ' })
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
  })
  it('only reads the clipboard on request and trims the preview payload', async () => {
    let reads = 0
    const wrapper = mount(LinkInput, { props: { ready: true, readClipboard: async () => { reads++; return '  商品 https://example.com/item  ' } } })
    expect(reads).toBe(0)
    await wrapper.get('[data-action=paste]').trigger('click')
    await flushPromises()
    expect(reads).toBe(1)
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('  商品 https://example.com/item  ')
    await wrapper.get('[data-action=preview]').trigger('click')
    expect(wrapper.emitted('preview')).toEqual([['商品 https://example.com/item']])
  })
  it('preserves typed input when clipboard access fails without reporting success', async () => {
    const wrapper = mount(LinkInput, { props: { ready: false, readClipboard: async () => { throw new Error('denied') } } })
    await wrapper.get('textarea').setValue('原始内容')
    await wrapper.get('[data-action=paste]').trigger('click')
    await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('原始内容')
    expect(wrapper.text()).toContain('无法读取剪贴板')
    expect(wrapper.text()).not.toContain('已粘贴')
  })
  it('blocks preview until actual prerequisites are ready, but permits local editing', async () => {
    const wrapper = mount(LinkInput, { props: { ready: false, readClipboard: async () => '' } })
    await wrapper.get('textarea').setValue('https://example.com/item')
    expect(wrapper.get('[data-action=preview]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-action=preview]').trigger('click')
    expect(wrapper.emitted('preview')).toBeUndefined()
    await wrapper.get('[data-action=clear]').trigger('click')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
  })
  it.each(['   ', 'x'.repeat(4097)])('rejects blank or oversized input before emitting a preview', async value => {
    const wrapper = mount(LinkInput, { props: { ready: true, readClipboard: async () => value } })
    await wrapper.get('[data-action=paste]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-action=preview]').trigger('click')
    expect(wrapper.emitted('preview')).toBeUndefined()
    expect(wrapper.text()).toMatch(/请输入|4096/)
  })
  it('accepts the exact input length boundary', async () => {
    const wrapper = mount(LinkInput, { props: { ready: true, readClipboard: async () => '中'.repeat(1365) + 'a' } })
    await wrapper.get('[data-action=paste]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-action=preview]').trigger('click')
    expect(wrapper.emitted('preview')?.[0]).toEqual(['中'.repeat(1365) + 'a'])
  })
  it('prevents repeated reads and preview or edits while clipboard reading is pending', async () => {
    let resolve!: (value: string) => void
    let reads = 0
    const wrapper = mount(LinkInput, { props: { ready: true, readClipboard: () => { reads++; return new Promise<string>(r => { resolve = r }) } } })
    await wrapper.get('[data-action=paste]').trigger('click')
    await wrapper.get('[data-action=paste]').trigger('click')
    expect(reads).toBe(1)
    for (const selector of ['textarea', '[data-action=paste]', '[data-action=clear]', '[data-action=preview]']) expect(wrapper.get(selector).attributes('disabled')).toBeDefined()
    resolve('https://example.com/item')
    await flushPromises()
    expect(wrapper.get('textarea').attributes('disabled')).toBeUndefined()
  })
})
