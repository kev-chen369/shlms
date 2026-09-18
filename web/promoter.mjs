import { mountPromotionFlow } from './promotion-flow-view.mjs';

const statusCopy = {
  NOT_APPLIED: ['尚未申请推广资格', '提交申请并通过审核后，才可创建推广位和生成专属链接。'],
  PENDING: ['申请审核中', '审核结果以推广中心状态为准，等待期间不能生成推广链接。'],
  REJECTED: ['申请未通过', '请查看审核原因，符合条件后可重新申请。'],
  DISABLED: ['推广资格已停用', '当前不能生成新链接；历史订单仍以平台回流结果为准。'],
  ENABLED: ['推广资格已启用', '可查看本人推广位和数据；渠道位核验前不能生成链接。'],
};

export async function loadPromoterState({ accessToken, request = fetch }) {
  if (!accessToken) return { kind: 'SIGNED_OUT' };
  const headers = { Authorization: `Bearer ${accessToken}` };
  try {
    const profileResponse = await request('/api/v1/promoter/profile', { headers, cache: 'no-store' });
    if (profileResponse.status === 401) return { kind: 'SIGNED_OUT' };
    if (!profileResponse.ok) return { kind: 'ERROR' };
    const profile = (await profileResponse.json())?.data;
    if (!profile || !Object.hasOwn(statusCopy, profile.status)) return { kind: 'ERROR' };
    if (profile.status !== 'ENABLED') return { kind: profile.status, reason: profile.reason || '' };
    const [positionsResult, dashboardResult] = await Promise.allSettled([
      request('/api/v1/promotion-positions?limit=20', { headers, cache: 'no-store' }),
      request('/api/v1/promoter/dashboard', { headers, cache: 'no-store' }),
    ]);
    const positionsResponse = positionsResult.status === 'fulfilled' ? positionsResult.value : null;
    const dashboardResponse = dashboardResult.status === 'fulfilled' ? dashboardResult.value : null;
    const positions = positionsResponse?.ok ? (await positionsResponse.json())?.data?.items : null;
    const dashboard = dashboardResponse?.ok ? (await dashboardResponse.json())?.data : null;
    return { kind: 'ENABLED', positions: Array.isArray(positions) ? positions : null, dashboard: dashboard && Number.isInteger(dashboard.successfulLinks) && Number.isInteger(dashboard.validOrders) ? dashboard : null };
  } catch {
    return { kind: 'ERROR' };
  }
}

function node(tag, className, text) {
  const item = document.createElement(tag);
  if (className) item.className = className;
  if (text !== undefined) item.textContent = text;
  return item;
}

function card(title, description) {
  const item = node('div', 'section-card promotion-card');
  item.append(node('h2', '', title), node('p', '', description));
  return item;
}

export function renderPromoterState(target, state, onBrowse, accessToken = null) {
  target.replaceChildren();
  if (state.kind === 'SIGNED_OUT') {
    const item = card('登录后查看推广资格', '登录接入完成后，可在这里查看申请状态、本人推广位和订单数据。');
    item.append(node('div', 'promotion-note', '当前不能提交推广申请或生成链接。'));
    const browse = node('button', 'promotion-button', '先浏览优惠');
    browse.type = 'button';
    browse.addEventListener('click', onBrowse);
    item.append(browse);
    target.append(item);
    return;
  }
  if (state.kind === 'ERROR') {
    target.append(card('暂时无法读取推广状态', '请稍后重试；当前不会开放转链操作。'));
    return;
  }
  const [title, description] = statusCopy[state.kind];
  const overview = card(title, description);
  if (state.reason && (state.kind === 'REJECTED' || state.kind === 'DISABLED')) overview.append(node('div', 'promotion-note', state.reason));
  target.append(overview);
  if (state.kind !== 'ENABLED') return;
  const positionCard = card('我的推广位', state.positions === null ? '推广位暂时无法读取' : state.positions.length ? '仅显示本人推广位；是否可用以渠道核验为准。' : '还没有推广位');
  if (state.positions?.length) {
    const list = node('ul', 'promotion-position-list');
    for (const position of state.positions) {
      const readiness = position.channels?.find((channel) => channel.channel === 'JD')?.readiness;
      list.append(node('li', '', `${position.name || '未命名推广位'} · ${readiness === 'WAITING_VERIFICATION' ? '渠道待核验' : readiness === 'READY' && position.canConvert ? '可用' : '渠道未就绪'}`));
    }
    positionCard.append(list);
  }
  target.append(positionCard);
  const dataCard = card('近期推广数据', state.dashboard ? `成功转链 ${state.dashboard.successfulLinks} 次 · 复制上报 ${state.dashboard.copyReports ?? 0} 次 · 有效归因订单 ${state.dashboard.validOrders} 笔` : '推广数据暂时无法读取');
  dataCard.append(node('small', '', '复制上报不代表送达；订单数不代表收益。'));
  target.append(dataCard);
  const convertCard = card('生成推广链接', '请先确认本人推广位及渠道状态。');
  if (!mountPromotionFlow(convertCard, { positions: state.positions || [], accessToken })) {
    convertCard.append(node('p', '', '渠道位尚未核验为可用，暂不能生成链接。'));
    const convert = node('button', 'promotion-button', '暂不可生成');
    convert.type = 'button';
    convert.disabled = true;
    convertCard.append(convert);
  }
  target.append(convertCard);
}
