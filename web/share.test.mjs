import assert from 'node:assert/strict';
import test from 'node:test';
import { copyPromotionShare } from './share.mjs';

const input = { requestId: 'conversion-1', artifactType: 'link', scene: 'home', accessToken: 'test-token', eventId: 'event-1' };
const artifact = { ok: true, json: async () => ({ data: { requestId: 'conversion-1', type: 'link', content: 'https://example.com/item' } }) };
const report = { ok: true, json: async () => ({ data: { eventId: 'event-1', requestId: 'conversion-1', action: 'COPY_REPORTED' } }) };

test('reports only after clipboard write resolves', async () => {
  const calls = [];
  const request = async (url, options) => { calls.push([url, options]); return calls.length === 1 ? artifact : report; };
  const clipboard = { writeText: async (text) => { assert.equal(text, 'https://example.com/item'); assert.equal(calls.length, 1); calls.push(['clipboard']); } };
  const result = await copyPromotionShare({ ...input, request, clipboard });
  assert.deepEqual(result, { copied: true, reported: true, eventId: 'event-1' });
  assert.equal(calls[2][1].method, 'POST');
  assert.equal(JSON.parse(calls[2][1].body).action, 'COPY_REPORTED');
});

test('clipboard rejection produces no report', async () => {
  let requests = 0;
  const request = async () => { requests++; return artifact; };
  const result = await copyPromotionShare({ ...input, request, clipboard: { writeText: async () => { throw new Error('denied'); } } });
  assert.deepEqual(result, { copied: false, reported: false, reason: 'COPY_FAILED' });
  assert.equal(requests, 1);
});

test('report failure keeps copy result distinct', async () => {
  let requests = 0;
  const request = async () => (++requests === 1 ? artifact : { ok: false });
  const result = await copyPromotionShare({ ...input, request, clipboard: { writeText: async () => {} } });
  assert.deepEqual(result, { copied: true, reported: false, reason: 'REPORT_FAILED' });
});

test('network failure after copy keeps copy result distinct', async () => {
  let requests = 0;
  const request = async () => { if (++requests === 1) return artifact; throw new Error('offline'); };
  const result = await copyPromotionShare({ ...input, request, clipboard: { writeText: async () => {} } });
  assert.deepEqual(result, { copied: true, reported: false, reason: 'REPORT_FAILED' });
});

test('unavailable or invalid artifact is never copied', async () => {
  let copied = false;
  const clipboard = { writeText: async () => { copied = true; } };
  for (const request of [async () => ({ ok: false }), async () => ({ ok: true, json: async () => ({ data: { ...await artifact.json().then((x) => x.data), requestId: 'other' } }) })]) {
    const result = await copyPromotionShare({ ...input, request, clipboard });
    assert.equal(result.copied, false);
  }
  assert.equal(copied, false);
});
