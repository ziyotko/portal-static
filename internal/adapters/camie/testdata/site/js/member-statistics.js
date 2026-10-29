(() => {
  'use strict';

  const target = document.querySelector('[data-member-statistics-dashboard]');
  const status = document.querySelector('[data-member-statistics-status]');
  if (!target || !status) return;

  const apiBase = '/business_portal/api/member-zone/statistics/dashboard';
  let dashboard;
  let requestId = 0;
  let token = '';
  try { token = localStorage.getItem('member-token') || ''; } catch { /* Storage disabled. */ }

  const login = () => {
    const url = new URL(document.body.dataset.memberLoginPath, location.href);
    url.searchParams.set('returnUrl', location.href);
    location.replace(url.href);
  };
  const message = (text) => { status.textContent = text; status.hidden = false; };
  const api = async (url) => {
    const response = await fetch(url, {
      headers: { Authorization: `Bearer ${token}` },
      credentials: 'same-origin',
      cache: 'no-store',
    });
    if (response.status === 401) {
      const error = new Error('登录已失效，请重新登录。');
      error.code = 401;
      throw error;
    }
    if (!response.ok) throw new Error('接口请求失败，请稍后重试。');
    const payload = await response.json();
    if (Number(payload.code) === 401) {
      const error = new Error('登录已失效，请重新登录。');
      error.code = 401;
      throw error;
    }
    if (Number(payload.code) !== 0) throw new Error(payload.message || '统计数据加载失败。');
    return payload.data;
  };
  const revenue = (wan) => Number(wan || 0) / 10000;
  const growth = (value) => value == null || value === '' ? null : Number(value);
  const mappedGroup = (item) => ({
    name: item.name || '',
    revenue: revenue(item.revenueYearWan),
    growth: growth(item.yoyPercent),
    count: Number(item.enterpriseCount || 0),
  });
  const mapDashboard = (data) => {
    const provinces = (data.provinces || []).map(mappedGroup);
    return {
      year: data.statisticsYear,
      comparisonYear: data.comparisonYear,
      updateDate: data.importedAt || '—',
      years: data.years || [],
      filters: data.filters || { regions: [], provinces: [], subFields: [] },
      recordCount: data.summary.recordCount,
      headline: {
        provinceCount: data.summary.provinceCount,
        companyCount: data.summary.enterpriseCount,
        revenue: revenue(data.summary.revenueYearWan),
        yoy: growth(data.summary.totalYoyPercent),
      },
      provinces: [...provinces].sort((a, b) => b.revenue - a.revenue),
      companies: [...provinces].sort((a, b) => b.count - a.count),
      segments: (data.subFields || []).map(mappedGroup).sort((a, b) => b.revenue - a.revenue),
      regions: (data.regions || []).map(mappedGroup).sort((a, b) => b.revenue - a.revenue),
    };
  };
  const load = async (filters = {}) => {
    const current = ++requestId;
    message('正在加载统计数据…');
    const query = new URLSearchParams();
    Object.entries(filters).forEach(([key, value]) => { if (value !== '') query.set(key, value); });
    try {
      const data = await api(`${apiBase}?${query}`);
      if (current !== requestId) return;
      if (!data?.statisticsYear) { target.hidden = true; message('数据中心暂无已发布的年度统计数据。'); return; }
      const mapped = mapDashboard(data);
      if (dashboard) dashboard.setData(mapped);
      else dashboard = window.mountEnvironmentDashboard(target, mapped, { onFilter: load });
      target.hidden = false;
      status.hidden = true;
    } catch (error) {
      if (current !== requestId) return;
      if (error.code === 401) { login(); return; }
      message(error.message || '统计数据加载失败，请稍后重试。');
    }
  };
  const begin = async () => {
    if (!token) { login(); return; }
    try {
      const profile = await api('/business_member/api/member/profile');
      if (profile?.status !== 'active') { message('当前账号不是正式会员，暂时无法查看数据中心。'); return; }
      await load();
    } catch (error) {
      if (error.code === 401) login();
      else message(error.message || '会员身份核验失败，请稍后重试。');
    }
  };
  void begin();
})();
