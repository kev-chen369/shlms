const platforms = [
  { id: 'JD', name: '京东', active: true },
  { id: 'MT', name: '美团', active: true },
  { id: 'TB', name: '淘宝', active: true },
  { id: 'PDD', name: '拼多多', active: false },
  { id: 'ELEME', name: '饿了么', active: false },
];

const state = { platform: 'JD', tab: 'home', request: null };
const byId = (id) => document.getElementById(id);
const platformName = () => platforms.find((item) => item.id === state.platform)?.name ?? '';
const yuan = (minor) => `¥${(minor / 100).toFixed(minor % 100 === 0 ? 0 : 2)}`;

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function statusBox(title, description, error = false) {
  const box = element('div', `status-box${error ? ' error' : ''}`);
  box.append(element('strong', '', title), element('span', '', description));
  if (error) {
    const retry = element('button', '', '重新加载');
    retry.type = 'button';
    retry.addEventListener('click', loadHomeCoupons);
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
  renderPlatforms();
  byId('home-title').textContent = `${platformName()}优惠专区`;
  byId('hero-description').textContent = platforms.find((p) => p.id === id)?.active
    ? '查看当前平台已核验的活动' : '渠道暂未开放';
  loadHomeCoupons();
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
    const response = await fetch(`/api/v1/coupons?platform=${encodeURIComponent(platform.id)}&limit=3`, { signal: controller.signal, cache: 'no-store' });
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
      const amount = element('span', 'mini-amount', yuan(item.discountMinor));
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
renderPlatforms();
selectTab('home');
loadHomeCoupons();
