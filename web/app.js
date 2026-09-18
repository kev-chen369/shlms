import { loadPromoterState, renderPromoterState } from './promoter.mjs';

const platforms = [
  { id: 'JD', name: '京东', active: true },
  { id: 'MT', name: '美团', active: true },
  { id: 'TB', name: '淘宝', active: true },
  { id: 'PDD', name: '拼多多', active: false },
  { id: 'ELEME', name: '饿了么', active: false },
];

const state = { platform: 'JD', cityCode: '', tab: 'home', accessToken: null, request: null, couponRequest: null, detailRequest: null, cityRequest: null, nextCursor: '', activeCoupon: '' };
const byId = (id) => document.getElementById(id);
const platformName = () => platforms.find((item) => item.id === state.platform)?.name ?? '';
const yuan = (minor) => `¥${(minor / 100).toFixed(minor % 100 === 0 ? 0 : 2)}`;

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function statusBox(title, description, error = false, retryAction = loadHomeCoupons) {
  const box = element('div', `status-box${error ? ' error' : ''}`);
  box.append(element('strong', '', title), element('span', '', description));
  if (error) {
    const retry = element('button', '', '重新加载');
    retry.type = 'button';
    retry.addEventListener('click', retryAction);
    box.append(retry);
  }
  return box;
}

function renderPlatforms() {
  const nav = byId('platforms');
  nav.replaceChildren();
  for (const platform of platforms) {
    const button = element('button', '', platform.name);
    button.type = 'button';
    button.setAttribute('aria-current', String(state.platform === platform.id));
    button.addEventListener('click', () => selectPlatform(platform.id));
    nav.append(button);
  }
}

function selectPlatform(id) {
  if (state.platform === id) return;
  state.platform = id;
  state.cityCode = '';
  renderPlatforms();
  byId('home-title').textContent = `${platformName()}优惠专区`;
  byId('hero-description').textContent = platforms.find((p) => p.id === id)?.active
    ? '查看当前平台已核验的活动' : '渠道暂未开放';
  loadHomeCoupons();
  closeDetail();
  loadCities();
  if (state.tab === 'coupons') loadCoupons();
}

async function loadCities() {
  state.cityRequest?.abort();
  const controller = new AbortController();
  state.cityRequest = controller;
  const select = byId('coupon-city');
  const hint = byId('city-hint');
  select.replaceChildren(element('option', '', '不限城市活动'));
  select.firstChild.value = '';
  hint.textContent = '正在读取可用城市…';
  if (!platforms.find((p) => p.id === state.platform)?.active) {
    hint.textContent = '渠道暂未开放';
    return;
  }
  try {
    const response = await fetch(`/api/v1/coupon-cities?platform=${encodeURIComponent(state.platform)}`, { signal: controller.signal, cache: 'no-store' });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const body = await response.json();
    if (controller.signal.aborted) return;
    const cities = body?.data?.items;
    if (!Array.isArray(cities)) throw new Error('Invalid cities response');
    for (const city of cities) {
      if (!city.code || !city.name) continue;
      const option = element('option', '', city.name);
      option.value = city.code;
      select.append(option);
    }
    hint.textContent = cities.length ? '选城市后仍包含不限城市活动' : '暂无已核验的城市专属活动';
  } catch (error) {
    if (error.name !== 'AbortError') hint.textContent = '城市选项暂不可用，仅展示不限城市活动';
  }
}

function selectCity(code) {
  state.cityCode = code;
  closeDetail();
  loadHomeCoupons();
  loadCoupons();
}

function selectTab(tab) {
  state.tab = tab;
  for (const name of ['home', 'coupons', 'promotion', 'mine']) {
    byId(`${name}-view`).hidden = name !== tab;
  }
  for (const button of document.querySelectorAll('.bottom-nav button')) {
    if (button.dataset.tab === tab) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  }
  byId('content').focus({ preventScroll: true });
  window.scrollTo({ top: 0, behavior: 'instant' });
  if (tab === 'coupons') loadCoupons();
  if (tab === 'promotion') refreshPromotion();
}

async function refreshPromotion() {
  const content = byId('promotion-content');
  content.replaceChildren(statusBox('正在检查推广状态', '请稍候…'));
  const result = await loadPromoterState({ accessToken: state.accessToken });
  if (state.tab !== 'promotion') return;
  renderPromoterState(content, result, () => selectTab('coupons'), state.accessToken);
}

function scopeName(scope) {
  return { PRODUCT: '指定商品', CATEGORY: '指定品类', SHOP: '指定店铺', ACTIVITY: '平台活动' }[scope] ?? '适用范围以平台规则为准';
}

function dateText(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间待核实';
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
}

function couponCard(item) {
  const card = element('article', 'coupon-card');
  const ticket = element('div', 'coupon-ticket');
  ticket.append(element('strong', '', item.discountMinor ? yuan(item.discountMinor) : '活动券'), element('span', '', item.thresholdMinor ? `满 ${yuan(item.thresholdMinor)} 可用` : '无门槛'));
  const info = element('div', 'coupon-info');
  info.append(element('strong', '', item.title), element('p', '', `${platformName()} · ${scopeName(item.scope)}`), element('small', '', `有效至 ${dateText(item.expiresAt)}`));
  const button = element('button', 'detail-button', '查看规则');
  button.type = 'button';
  button.addEventListener('click', () => loadDetail(item.id));
  card.append(ticket, info, button);
  return card;
}

async function loadCoupons(reset = true) {
  state.couponRequest?.abort();
  const controller = new AbortController();
  state.couponRequest = controller;
  const target = byId('coupon-list');
  const more = byId('coupon-more');
  const platform = platforms.find((p) => p.id === state.platform);
  byId('coupon-subtitle').textContent = state.cityCode
    ? `正在查看${byId('coupon-city').selectedOptions[0].textContent}及不限城市的已核验活动` : '当前只展示不限城市的已核验活动';
  if (reset) {
    state.nextCursor = '';
    target.replaceChildren();
  }
  more.hidden = true;
  if (!platform.active) {
    target.replaceChildren(statusBox('渠道暂未开放', `${platform.name}的真实券活动还未接通`));
    return;
  }
  if (reset) target.replaceChildren(statusBox('正在读取优惠', '正在核对当前平台的券活动…'));
  else {
    target.querySelector('.status-box.error')?.remove();
    more.textContent = '加载中…';
  }
  const cursor = state.nextCursor;
  try {
    const query = new URLSearchParams({ platform: platform.id, limit: '20' });
    if (state.cityCode) query.set('cityCode', state.cityCode);
    if (cursor) query.set('cursor', cursor);
    const response = await fetch(`/api/v1/coupons?${query}`, { signal: controller.signal, cache: 'no-store' });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const body = await response.json();
    if (controller.signal.aborted) return;
    const items = body?.data?.items;
    if (!Array.isArray(items) || typeof body.data.nextCursor !== 'string') throw new Error('Invalid catalog response');
    if (reset) target.replaceChildren();
    if (items.length === 0 && reset) target.append(statusBox('暂无可用优惠', '有已核验的券活动时会在这里展示'));
    else target.append(...items.map(couponCard));
    state.nextCursor = body.data.nextCursor;
    more.hidden = !state.nextCursor;
    more.textContent = '加载更多优惠';
  } catch (error) {
    if (error.name === 'AbortError') return;
    if (reset) target.replaceChildren(statusBox('暂时无法读取优惠', '请稍后重试；目前没有可确认的券活动', true, () => loadCoupons()));
    else target.append(statusBox('加载更多失败', '请稍后重试', true, () => loadCoupons(false)));
    more.textContent = '加载更多优惠';
  }
}

function closeDetail() {
  state.detailRequest?.abort();
  state.activeCoupon = '';
  byId('coupon-detail').hidden = true;
  byId('detail-content').replaceChildren();
}

async function loadDetail(id) {
  state.detailRequest?.abort();
  const controller = new AbortController();
  state.detailRequest = controller;
  state.activeCoupon = id;
  byId('coupon-detail').hidden = false;
  byId('detail-content').replaceChildren(statusBox('正在读取规则', '正在核对券详情…'));
  byId('coupon-detail').scrollIntoView({ behavior: 'smooth', block: 'start' });
  try {
    const context = state.cityCode ? `?cityCode=${encodeURIComponent(state.cityCode)}` : '';
    const response = await fetch(`/api/v1/coupons/${encodeURIComponent(id)}${context}`, { signal: controller.signal, cache: 'no-store' });
    if (controller.signal.aborted || state.activeCoupon !== id) return;
    if (response.status === 404) {
      byId('detail-content').replaceChildren(statusBox('活动已失效', '请返回列表查看其他优惠'));
      return;
    }
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const body = await response.json();
    if (controller.signal.aborted || state.activeCoupon !== id || !body?.data || body.data.id !== id) return;
    const item = body.data;
    const content = byId('detail-content');
    const title = element('h3', '', item.title);
    const detail = element('dl', 'detail-grid');
    const fields = [
      ['所属平台', platformName()], ['优惠金额', yuan(item.discountMinor)],
      ['使用门槛', item.thresholdMinor ? `满 ${yuan(item.thresholdMinor)}` : '无门槛'],
      ['适用范围', `${scopeName(item.scope)}${item.scopeName ? ` · ${item.scopeName}` : ''}`],
      ['活动有效期', `至 ${dateText(item.expiresAt)}`], ['数据更新', dateText(item.updatedAt)],
      ['领取方式', item.actionLabel || '以平台页面为准'],
    ];
    if (item.cityCode) fields.push(['适用城市', item.cityName || item.cityCode]);
    if (item.business) fields.push(['适用业务', item.business]);
    for (const [label, value] of fields) detail.append(element('dt', '', label), element('dd', '', value));
    const notice = element('p', 'detail-notice', '领券与平台跳转尚未接通。请勿根据这里的展示判断券已领取；最终优惠以平台结算页为准。');
    content.replaceChildren(title, detail, notice);
    if (item.scope !== 'ACTIVITY') loadProducts(id, controller.signal);
  } catch (error) {
    if (error.name !== 'AbortError') byId('detail-content').replaceChildren(statusBox('暂时无法读取规则', '请稍后再试', true, () => loadDetail(id)));
  }
}

async function loadProducts(id, signal) {
  const content = byId('detail-content');
  const section = element('section', 'product-scope');
  section.append(element('h4', '', '适用商品'));
  const body = element('div', '', '正在读取商品范围…');
  section.append(body);
  content.append(section);
  try {
    const query = new URLSearchParams({ limit: '20' });
    if (state.cityCode) query.set('cityCode', state.cityCode);
    const response = await fetch(`/api/v1/coupons/${encodeURIComponent(id)}/products?${query}`, { signal, cache: 'no-store' });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const result = await response.json();
    if (signal.aborted || state.activeCoupon !== id) return;
    const items = result?.data?.items;
    if (!Array.isArray(items)) throw new Error('Invalid products response');
    if (!items.length) body.textContent = '暂无已核验的适用商品，请以平台规则为准';
    else {
      const list = element('ul');
      for (const product of items) list.append(element('li', '', product.title));
      body.replaceChildren(list);
      if (result.data.nextCursor) body.append(element('p', 'scope-note', '还有更多适用商品，完整范围以平台规则为准'));
    }
  } catch (error) {
    if (error.name !== 'AbortError') body.textContent = '暂时无法读取适用商品';
  }
}

async function loadHomeCoupons() {
  state.request?.abort();
  const controller = new AbortController();
  state.request = controller;
  const target = byId('home-coupons');
  const platform = platforms.find((p) => p.id === state.platform);
  if (!platform.active) {
    target.replaceChildren(statusBox('渠道暂未开放', `${platform.name}的真实券活动还未接通`));
    return;
  }
  target.replaceChildren(statusBox('正在读取优惠', '正在核对当前平台的券活动…'));
  try {
    const query = new URLSearchParams({ platform: platform.id, limit: '3' });
    if (state.cityCode) query.set('cityCode', state.cityCode);
    const response = await fetch(`/api/v1/coupons?${query}`, { signal: controller.signal, cache: 'no-store' });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const body = await response.json();
    if (controller.signal.aborted) return;
    const items = body?.data?.items;
    if (!Array.isArray(items)) throw new Error('Invalid catalog response');
    if (items.length === 0) {
      target.replaceChildren(statusBox('暂无可领取的真实活动', '有已核验的优惠券时会在这里展示'));
      return;
    }
    target.replaceChildren(...items.map((item) => {
      const card = element('div', 'mini-coupon');
      const amount = element('span', 'mini-amount', item.discountMinor ? yuan(item.discountMinor) : '活动券');
      const content = element('div');
      content.append(element('strong', '', item.title), element('p', '', `满 ${yuan(item.thresholdMinor)} 可用 · ${item.actionLabel}`));
      card.append(amount, content);
      return card;
    }));
  } catch (error) {
    if (error.name !== 'AbortError') target.replaceChildren(statusBox('暂时无法读取优惠', '请稍后重试；目前没有可确认的券活动', true));
  }
}

document.querySelectorAll('[data-tab]').forEach((button) => button.addEventListener('click', () => selectTab(button.dataset.tab)));
byId('coupon-more').addEventListener('click', () => loadCoupons(false));
byId('detail-close').addEventListener('click', closeDetail);
byId('coupon-city').addEventListener('change', (event) => selectCity(event.target.value));
renderPlatforms();
selectTab('home');
loadCities();
loadHomeCoupons();
