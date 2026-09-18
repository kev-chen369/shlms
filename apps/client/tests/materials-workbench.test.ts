import { ref } from 'vue'
import { afterEach, expect, it } from 'vitest'
import { createMaterialsWorkbench } from '../src/features/promotion/materials-workbench'
const position = { id: 'p1', name: '社群位', scene: 'group', status: 'ENABLED', isDefault: true, canConvert: false, channels: [{ channel: 'JD', readiness: 'WAITING_CONFIGURATION' }] }
const cleanup: (() => void)[] = []
afterEach(() => cleanup.splice(0).forEach(fn => fn()))
function harness(token: string | null = 'identity') {
  const session = ref(token), calls: string[] = []; let status = 200, items = [position], nextCursor = ''
  const model = createMaterialsWorkbench(session, 'H5', async url => {
    calls.push(url)
    return { statusCode: status, data: { code: 0, data: url.includes('/promotion-positions') ? { items, nextCursor } : { items: [], capability: { allowed: false, reason: 'UNCONFIGURED' } } } }
  })
  cleanup.push(model.dispose)
  return { model, session, calls, status: (v: number) => { status = v }, positions: (v: typeof items, cursor = '') => { items = v; nextCursor = cursor } }
}
it('signed out makes no position or catalog request', async () => {
  const { model, calls } = harness(null); await model.refreshPositions(); model.choose('p1'); model.apply('', '')
  expect(calls).toEqual([]); expect(model.state.status).toBe('login-required')
})
it('does not auto-select default position and requires explicit scene confirmation', async () => {
  const { model, calls } = harness(); await model.refreshPositions()
  expect(model.state.selectedId).toBe(''); expect(model.materials.state.scope).toBeNull()
  model.choose('p1'); expect(model.state.scene).toBe('group'); expect(calls).toHaveLength(1)
  model.apply('310100', 'food'); await new Promise(resolve => setTimeout(resolve, 0))
  expect(calls[1]).toContain('positionId=p1&scene=group&cityCode=310100&business=food')
  expect(model.materials.state.status).toBe('blocked')
})
it('rejects forged selection and byte-incompatible legal positions with explanations', async () => {
  const { model, calls, positions } = harness(); positions([{ ...position, scene: '中'.repeat(80) }]); await model.refreshPositions()
  expect(model.state.items).toHaveLength(1); model.choose('p1'); model.apply('', '')
  expect(model.state.error).toContain('范围'); expect(calls).toHaveLength(1)
  model.choose('forged'); expect(model.state.selectedId).toBe(''); expect(model.materials.state.scope).toBeNull()
})
it('platform and type changes clear selected position and catalog scope', async () => {
  const { model, calls } = harness(); await model.refreshPositions(); model.choose('p1'); model.apply('', '')
  model.materials.selectType('ACTIVITY'); expect(model.state.selectedId).toBe(''); expect(model.materials.state.scope).toBeNull()
  model.materials.selectPlatform('MEITUAN'); await model.refreshPositions()
  expect(model.state.status).toBe('unavailable'); expect(model.state.items).toEqual([]); expect(calls.filter(url => url.includes('/promotion-positions'))).toHaveLength(1)
})
it('failed pagination retains choices and can explicitly recover', async () => {
  const { model, status, positions } = harness(); positions([position], 'YQ'); await model.refreshPositions()
  status(503); await model.morePositions(); expect(model.state.items).toHaveLength(1); expect(model.state.pageError).not.toBe('')
  status(200); positions([{ ...position, id: 'p2' }]); await model.morePositions()
  expect(model.state.items.map(p => p.id)).toEqual(['p1', 'p2']); expect(model.state.pageError).toBe('')
})
it('401 clears both choices and scope and prevents retry with rejected session', async () => {
  const { model, status, calls } = harness(); await model.refreshPositions(); model.choose('p1'); status(401)
  await model.refreshPositions(); await model.refreshPositions()
  expect(model.state.status).toBe('login-required'); expect(model.state.items).toEqual([])
  expect(model.materials.state.scope).toBeNull(); expect(calls).toHaveLength(2)
})
it('session change clears private choices and does not auto-request', async () => {
  const { model, session, calls } = harness(); await model.refreshPositions(); model.choose('p1'); session.value = 'new-identity'
  expect(model.state.items).toEqual([]); expect(model.state.selectedId).toBe(''); expect(calls).toHaveLength(1)
})
it('ignores old session ABA responses', async () => {
  const session = ref<string | null>('a'); let resolve!: (v: unknown) => void
  const model = createMaterialsWorkbench(session, 'H5', () => new Promise(done => { resolve = done as typeof resolve })); cleanup.push(model.dispose)
  const pending = model.refreshPositions(); session.value = 'b'; session.value = 'a'
  resolve({ statusCode: 200, data: { code: 0, data: { items: [position] } } }); await pending
  expect(model.state.items).toEqual([])
})
it('validates city/business without discarding valid position or requesting', async () => {
  const { model, calls } = harness(); await model.refreshPositions(); model.choose('p1'); model.apply('中'.repeat(11), '')
  expect(calls).toHaveLength(1); expect(model.state.selectedId).toBe('p1'); expect(model.state.error).not.toBe('')
})
it.each([false, true])('late 401 only locks its own authentication epoch (changed=%s)', async changed => {
  const session = ref('first'); let resolve!: (value: { statusCode: number; data: unknown }) => void
  let calls = 0
  const model = createMaterialsWorkbench(session, 'H5', async () => {
    if (++calls === 1) return new Promise(done => { resolve = done })
    return { statusCode: 200, data: { code: 0, data: { items: [position] } } }
  }); cleanup.push(model.dispose)
  const pending = model.refreshPositions()
  if (changed) { session.value = 'second'; await model.refreshPositions() }
  else model.materials.selectType('ACTIVITY')
  resolve({ statusCode: 401, data: { message: 'private' } }); await pending
  expect(model.state.status).toBe(changed ? 'ready' : 'login-required')
  expect(model.state.items.length).toBe(changed ? 1 : 0)
})
it('catalog 401 clears position choices and blocks same-session position retries', async () => {
  const { model, calls, status } = harness(); await model.refreshPositions(); model.choose('p1'); status(401); model.apply('', '')
  await new Promise(resolve => setTimeout(resolve, 0)); await model.refreshPositions()
  expect(model.state.status).toBe('login-required'); expect(model.state.items).toEqual([]); expect(calls).toHaveLength(2)
})
