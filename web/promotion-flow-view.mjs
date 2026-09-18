import { createPromotionFlow } from './promotion-flow.mjs';

const make = (tag, text, className = '') => {
  const item = document.createElement(tag);
  if (text !== undefined) item.textContent = text;
  if (className) item.className = className;
  return item;
};

export function mountPromotionFlow(target, { positions, accessToken, request = fetch, clipboard = navigator.clipboard }) {
  const ready = positions.filter((p) => p.status === 'ENABLED' && p.canConvert && p.channels?.some((c) => c.channel === 'JD' && c.readiness === 'READY'));
  if (!accessToken || !ready.length) return false;
  const form = make('div', undefined, 'promotion-flow');
  const positionLabel = make('label', '推广位');
  const select = make('select');
  for (const p of ready) { const option = make('option', p.name); option.value = p.id; select.append(option); }
  positionLabel.append(select);
  const inputLabel = make('label', '粘贴京东商品链接');
  const input = make('textarea');
  input.placeholder = '请主动粘贴已支持的商品链接';
  input.rows = 3;
  input.maxLength = 4096;
  inputLabel.append(input);
  const result = make('div', undefined, 'promotion-flow-result');
  form.append(positionLabel, inputLabel, result);
  target.append(form);
  let flow;
  const action = (label, handler, disabled = false) => {
    const button = make('button', label, 'promotion-button');
    button.type = 'button'; button.disabled = disabled;
    button.addEventListener('click', handler);
    result.append(button);
  };
  const render = (s) => {
    result.replaceChildren();
    const locked = ['CONVERTING', 'CONVERT_RETRY', 'PENDING', 'PROCESSING', 'FAILED_RETRYABLE', 'QUERYING', 'QUERY_REQUIRED'].includes(s.stage);
    select.disabled = locked;
    input.disabled = locked;
    const message = {
      EDITING: '确认商品前不会生成推广链接。', PREVIEWING: '正在读取商品与价格规则…',
      PREVIEW_RETRY: '预览结果未确认，可用同一请求重试。', STALE: '商品或规则已变化，请重新预览。',
      CONFIRM: '请核对商品、价格和预计收益，再生成专属链接。', CONVERTING: '正在提交转链请求…',
      CONVERT_RETRY: '提交结果不确定，请用同一请求重试，不要重新创建。',
      PENDING: '链接生成中，请查询原请求。', PROCESSING: '链接生成中，请查询原请求。',
      FAILED_RETRYABLE: '渠道结果待确认，请查询原请求。', QUERYING: '正在查询原请求…',
      QUERY_REQUIRED: '暂时无法确认结果，请继续查询原请求。',
      FAILED_FINAL: '转链未成功，请重新预览后再试。', SUCCEEDED: '链接已生成，可复制链接或文案。',
    }[s.stage] || '当前状态暂不可用';
    result.append(make('p', message));
    if (s.stage === 'CONFIRM' && s.preview) {
      const detail = make('div', undefined, 'promotion-flow-preview');
      detail.append(make('strong', s.preview.product?.name || '商品名称待核验'));
      const yuan = (value) => Number.isInteger(value) ? `¥${(value / 100).toFixed(2)}` : '待核验';
      detail.append(make('span', `券后价 ${yuan(s.preview.couponPriceMinor)} · 本人预计收益 ${yuan(s.preview.promoterEstimateMinor)} · 消费者预计返现 ${yuan(s.preview.consumerCashbackEstimateMinor)}`));
      detail.append(make('small', '均为预估，最终以渠道与平台结算规则为准。'));
      result.append(detail);
    }
    if (['EDITING', 'PREVIEW_RETRY', 'STALE'].includes(s.stage)) action('预览商品', () => flow.preview(), !input.value.trim());
    if (['CONFIRM', 'CONVERT_RETRY'].includes(s.stage)) action(s.stage === 'CONFIRM' ? '确认生成链接' : '使用原请求重试', () => flow.convert());
    if (['PENDING', 'PROCESSING', 'FAILED_RETRYABLE', 'QUERY_REQUIRED'].includes(s.stage)) action('查询生成结果', () => flow.refresh());
    if (s.stage === 'FAILED_FINAL') action('重新预览', () => flow.restart());
    if (s.stage === 'SUCCEEDED') {
      action('复制链接', () => flow.copy('link'));
      action('复制文案', () => flow.copy('text'));
      action('二维码暂未开放', () => {}, true);
      action('海报暂未开放', () => {}, true);
      if (s.copyMessage) result.append(make('p', s.copyMessage, 'promotion-copy-message'));
    }
  };
  flow = createPromotionFlow({ accessToken, request, clipboard, onChange: render });
  flow.setPosition(ready[0].id);
  flow.setScene(ready[0].scene || 'home');
  select.addEventListener('change', () => {
    flow.setPosition(select.value);
    flow.setScene(ready.find((position) => position.id === select.value)?.scene || 'home');
  });
  input.addEventListener('input', () => flow.setInput(input.value));
  render(flow.snapshot());
  return true;
}
