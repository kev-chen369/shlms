import { copyPromotionShare } from './share.mjs';

const safeId = (value) => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const path = (id) => `/api/v1/promotions/convert/${encodeURIComponent(id)}`;

export function createPromotionFlow({ accessToken, request = fetch, clipboard = globalThis.navigator?.clipboard, newId = () => crypto.randomUUID(), onChange = () => {} }) {
  if (!accessToken) throw new Error('AUTH_REQUIRED');
  const state = { stage: 'EDITING', input: '', positionId: '', scene: 'home', preview: null, previewKey: '', convertKey: '', requestId: '', copyMessage: '' };
  let busy = false;
  const headers = { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' };
  const emit = () => onChange({ ...state });
  const reset = () => { state.preview = null; state.previewKey = ''; state.convertKey = ''; state.requestId = ''; state.copyMessage = ''; state.stage = 'EDITING'; emit(); };
  const locked = () => ['CONVERTING', 'CONVERT_RETRY', 'PENDING', 'PROCESSING', 'FAILED_RETRYABLE', 'QUERYING', 'QUERY_REQUIRED'].includes(state.stage);
  const api = async (url, options) => {
    const response = await request(url, { ...options, headers: options?.headers || headers, cache: 'no-store' });
    return { response, body: await response.json() };
  };
  const flow = {
    snapshot: () => ({ ...state }),
    restart() { if (!busy) reset(); },
    setInput(value) { if (busy || locked()) return; if (value !== state.input) { state.input = value; reset(); } },
    setPosition(value) { if (busy || locked()) return; if (value !== state.positionId) { state.positionId = value; reset(); } },
    setScene(value) { if (busy || locked()) return; if (value !== state.scene) { state.scene = value; reset(); } },
    async preview() {
      if (busy || !state.input.trim() || state.input.length > 4096 || !safeId(state.positionId) || !safeId(state.scene)) return;
      busy = true; state.stage = 'PREVIEWING'; emit();
      state.previewKey ||= newId();
      try {
        const { response, body } = await api('/api/v1/promotions/preview', { method: 'POST', headers: { ...headers, 'Idempotency-Key': state.previewKey }, body: JSON.stringify({ input: state.input.trim(), positionId: state.positionId, scene: state.scene }) });
        const data = body?.data;
        if (!response.ok || !safeId(data?.previewId) || data.positionId !== state.positionId || data.scene !== state.scene || data.product?.channel !== 'JD' || !Number.isFinite(Date.parse(data.expiresAt))) {
          state.stage = response.status === 409 ? 'STALE' : 'PREVIEW_RETRY';
          if (state.stage === 'STALE') state.previewKey = '';
        } else { state.preview = data; state.stage = 'CONFIRM'; }
      } catch { state.stage = 'PREVIEW_RETRY'; }
      finally { busy = false; emit(); }
    },
    async convert() {
      if (busy || state.stage !== 'CONFIRM' && state.stage !== 'CONVERT_RETRY' || !state.preview || Date.parse(state.preview.expiresAt) <= Date.now()) { if (state.preview && Date.parse(state.preview.expiresAt) <= Date.now()) { state.stage = 'STALE'; emit(); } return; }
      busy = true; state.stage = 'CONVERTING'; emit();
      state.convertKey ||= newId();
      try {
        const { response, body } = await api('/api/v1/promotions/convert', { method: 'POST', headers: { ...headers, 'Idempotency-Key': state.convertKey }, body: JSON.stringify({ previewId: state.preview.previewId, positionId: state.positionId, scene: state.scene }) });
        const data = body?.data;
        if (response.ok && safeId(data?.requestId) && ['PENDING', 'PROCESSING', 'FAILED_RETRYABLE', 'FAILED_FINAL', 'SUCCEEDED'].includes(data.status)) {
          state.requestId = data.requestId;
          state.stage = data.status;
        } else if (response.status === 409) { state.stage = 'STALE'; state.preview = null; state.convertKey = ''; }
        else state.stage = 'CONVERT_RETRY';
      } catch { state.stage = 'CONVERT_RETRY'; }
      finally { busy = false; emit(); }
    },
    async refresh() {
      if (busy || !safeId(state.requestId)) return;
      busy = true; const previous = state.stage; state.stage = 'QUERYING'; emit();
      try {
        const { response, body } = await api(path(state.requestId), { headers: { Authorization: `Bearer ${accessToken}` } });
        const data = body?.data;
        state.stage = response.ok && data?.requestId === state.requestId && ['PENDING', 'PROCESSING', 'FAILED_RETRYABLE', 'FAILED_FINAL', 'SUCCEEDED'].includes(data.status) ? data.status : 'QUERY_REQUIRED';
      } catch { state.stage = 'QUERY_REQUIRED'; }
      finally { busy = false; if (state.stage === 'QUERY_REQUIRED' && previous === 'SUCCEEDED') state.stage = 'SUCCEEDED'; emit(); }
    },
    async copy(type) {
      if (busy || state.stage !== 'SUCCEEDED' || !['link', 'text'].includes(type)) return;
      busy = true;
      try {
        const result = await copyPromotionShare({ requestId: state.requestId, artifactType: type, scene: state.scene, accessToken, clipboard, request, eventId: newId() });
        state.copyMessage = result.copied ? result.reported ? '已复制；复制操作已记录' : '已复制；操作记录暂不可用' : '复制未完成，请重试';
      } catch { state.copyMessage = '复制未完成，请重试'; }
      finally { busy = false; emit(); }
    },
  };
  return flow;
}
