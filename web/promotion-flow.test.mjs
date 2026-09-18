import assert from 'node:assert/strict';
import test from 'node:test';
import { createPromotionFlow } from './promotion-flow.mjs';

const reply = (data, status = 200) => ({ ok: status >= 200 && status < 300, status, json: async () => ({ data }) });
const preview = () => ({ previewId: 'pv-1', positionId: 'pos-1', scene: 'home', product: { channel: 'JD', name: '商品' }, couponPriceMinor: 1000, promoterEstimateMinor: 100, consumerCashbackEstimateMinor: 50, expiresAt: new Date(Date.now() + 60000).toISOString() });

test('preview, confirm, query and copy never treat pending as success', async () => {
  const calls = [];
  const responses = [reply(preview()), reply({ requestId: 'cr-1', status: 'PENDING' }, 202), reply({ requestId: 'cr-1', status: 'SUCCEEDED' }), reply({ requestId: 'cr-1', type: 'link', content: 'https://example.com/item' }), reply({ requestId: 'cr-1', eventId: 'event-1', action: 'COPY_REPORTED' })];
  const request = async (url, options) => { calls.push([url, options]); return responses.shift(); };
  const copied = [];
  const ids = ['preview-key', 'convert-key', 'event-1'];
  const flow = createPromotionFlow({ accessToken: 'token', request, clipboard: { writeText: async (text) => copied.push(text) }, newId: () => ids.shift() });
  flow.setInput('https://item.jd.com/123.html'); flow.setPosition('pos-1');
  await flow.preview(); assert.equal(flow.snapshot().stage, 'CONFIRM');
  await flow.convert(); assert.equal(flow.snapshot().stage, 'PENDING'); assert.equal(copied.length, 0);
  await flow.copy('link'); assert.equal(calls.length, 2);
  await flow.refresh(); assert.equal(flow.snapshot().stage, 'SUCCEEDED');
  await flow.copy('link'); assert.deepEqual(copied, ['https://example.com/item']);
  assert.equal(calls[1][1].headers['Idempotency-Key'], 'convert-key');
  assert.equal(calls[4][1].method, 'POST');
});

test('uncertain conversion retains idempotency key for manual retry', async () => {
  const keys = [];
  let converts = 0;
  const request = async (url, options) => {
    if (url.endsWith('/preview')) return reply(preview());
    keys.push(options.headers['Idempotency-Key']);
    if (++converts === 1) throw new Error('timeout');
    return reply({ requestId: 'cr-1', status: 'PENDING' }, 202);
  };
  const flow = createPromotionFlow({ accessToken: 'token', request, newId: () => 'stable-id' });
  flow.setInput('product'); flow.setPosition('pos-1'); await flow.preview(); await flow.convert();
  assert.equal(flow.snapshot().stage, 'CONVERT_RETRY');
  flow.setInput('another product'); flow.setPosition('another-position');
  assert.equal(flow.snapshot().input, 'product');
  assert.equal(flow.snapshot().positionId, 'pos-1');
  await flow.convert(); assert.equal(flow.snapshot().stage, 'PENDING');
  assert.deepEqual(keys, ['stable-id', 'stable-id']);
});

test('changing input or position invalidates old preview and result', async () => {
  const flow = createPromotionFlow({ accessToken: 'token', request: async () => reply(preview()), newId: () => 'key' });
  flow.setInput('a'); flow.setPosition('pos-1'); await flow.preview();
  flow.setPosition('pos-2'); assert.equal(flow.snapshot().preview, null); assert.equal(flow.snapshot().stage, 'EDITING');
  flow.setInput('b'); assert.equal(flow.snapshot().previewKey, '');
});

test('expired preview prevents conversion request', async () => {
  let calls = 0;
  const request = async () => { calls++; return reply({ ...preview(), expiresAt: new Date(Date.now() - 1000).toISOString() }); };
  const flow = createPromotionFlow({ accessToken: 'token', request, newId: () => 'key' });
  flow.setInput('a'); flow.setPosition('pos-1'); await flow.preview(); await flow.convert();
  assert.equal(flow.snapshot().stage, 'STALE'); assert.equal(calls, 1);
});
