import assert from 'node:assert/strict';
import test from 'node:test';
import { loadPromoterState } from './promoter.mjs';

const response = (data, status = 200) => ({ ok: status === 200, status, json: async () => ({ data }) });

test('signed-out state makes no API call', async () => {
  const state = await loadPromoterState({ accessToken: null, request: () => { throw new Error('unexpected request'); } });
  assert.deepEqual(state, { kind: 'SIGNED_OUT' });
});

test('all membership states come from authenticated profile', async () => {
  for (const kind of ['NOT_APPLIED', 'PENDING', 'REJECTED', 'DISABLED']) {
    const state = await loadPromoterState({ accessToken: 'test', request: async (url, options) => {
      assert.equal(url, '/api/v1/promoter/profile');
      assert.equal(options.headers.Authorization, 'Bearer test');
      return response({ status: kind, reason: 'visible reason' });
    } });
    assert.equal(state.kind, kind);
  }
});

test('enabled state reads own positions and count-only dashboard', async () => {
  const calls = [];
  const state = await loadPromoterState({ accessToken: 'test', request: async (url) => {
    calls.push(url);
    if (url === '/api/v1/promoter/profile') return response({ status: 'ENABLED' });
    if (url.startsWith('/api/v1/promotion-positions')) return response({ items: [{ name: '主位', channels: [{ channel: 'JD', readiness: 'WAITING_VERIFICATION' }], canConvert: false }] });
    return response({ successfulLinks: 0, copyReports: 0, validOrders: 0 });
  } });
  assert.equal(state.kind, 'ENABLED');
  assert.equal(state.positions.length, 1);
  assert.equal(state.dashboard.validOrders, 0);
  assert.equal(calls.length, 3);
});

test('unauthorized and failed profile do not expose workbench', async () => {
  assert.deepEqual(await loadPromoterState({ accessToken: 'bad', request: async () => response(null, 401) }), { kind: 'SIGNED_OUT' });
  assert.deepEqual(await loadPromoterState({ accessToken: 'test', request: async () => { throw new Error('offline'); } }), { kind: 'ERROR' });
});
