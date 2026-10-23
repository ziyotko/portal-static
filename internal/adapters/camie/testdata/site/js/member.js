(() => {
  const listShell = document.querySelector('[data-member-list-shell]');
  const detailShell = document.querySelector('[data-member-detail-shell]');
  const shell = listShell || detailShell;
  if (!shell) return;

  const apiBase = '/business_portal/api/member-zone';
  const filePrefix = '/business_portal/api/member-files/';
  const status = shell.querySelector('[data-member-status]');
  const breadcrumb = shell.querySelector('[data-member-breadcrumb]');
  const params = new URLSearchParams(location.search);
  const typeNames = { 1: '新闻', 2: '数据', 3: '视频', 4: '报刊' };
  const memberDashboardPath = '/business_member/member/dashboard';
  let token = '';

  const node = (tag, className, value) => {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (value != null) element.textContent = String(value);
    return element;
  };
  const link = (label, href, className) => {
    const element = node('a', className, label);
    element.href = href;
    return element;
  };
  const setStatus = (message, action) => {
    status.replaceChildren(node('p', '', message));
    if (action) status.append(link(action.label, action.href, 'member-action'));
    status.hidden = false;
  };
  const loginURL = () => {
    const login = new URL(document.body.dataset.memberLoginPath, location.href);
    login.searchParams.set('returnUrl', location.href);
    return login.href;
  };
  const redirectToLogin = () => location.replace(loginURL());
  const memberSpaceItem = () => {
    let storedToken = '';
    try { storedToken = localStorage.getItem('member-token') || ''; } catch { storedToken = ''; }
    let href = memberDashboardPath;
    if (!storedToken) {
      const login = new URL(document.body.dataset.memberLoginPath, location.href);
      login.searchParams.set('returnUrl', new URL(memberDashboardPath, location.origin).href);
      href = login.href;
    }
    const li = node('li', 'member-space-item');
    const a = link('我的空间', href);
    a.dataset.memberSpaceLink = '';
    li.append(a);
    return li;
  };
  const api = async (path) => {
    let response;
    try {
      response = await fetch(path, {
        headers: { Authorization: `Bearer ${token}` },
        credentials: 'same-origin',
        cache: 'no-store',
      });
    } catch {
      throw new Error('网络异常，请检查连接后重试。');
    }
    if (!response.ok) throw new Error('接口请求失败，请稍后重试。');
    let payload;
    try { payload = await response.json(); } catch { throw new Error('接口返回异常，请稍后重试。'); }
    if (payload.code === 401) {
      const error = new Error('登录已失效，请重新登录。');
      error.code = 401;
      throw error;
    }
    if (payload.code !== 0) {
      const error = new Error(typeof payload.message === 'string' ? payload.message : '接口请求失败，请稍后重试。');
      error.code = payload.code;
      throw error;
    }
    return payload.data;
  };
  const verifiedProfile = async () => {
    const profile = await api('/business_member/api/member/profile');
    return profile?.status === 'active';
  };
  const begin = async () => {
    try { token = localStorage.getItem('member-token') || ''; } catch { token = ''; }
    if (!token) { redirectToLogin(); return false; }
    try {
      if (!(await verifiedProfile())) {
        setStatus('当前账号不是正式会员，暂时无法查看会员内容。', {
          label: '进入会员中心', href: '/business_member/member/dashboard',
        });
        return false;
      }
    } catch (error) {
      if (error.code === 401) redirectToLogin();
      else setStatus(error.message || '会员身份核验失败，请稍后重试。');
      return false;
    }
    return true;
  };
  const handleError = (error, fallback) => {
    if (error.code === 401) redirectToLogin();
    else setStatus(error.message || fallback);
  };
  const setBreadcrumb = (column, title) => {
    const items = [link('首页', '../index.html'), link('会员中心', 'member.html')];
    if (column) items.push(link(column, `member.html?column=${encodeURIComponent(column)}`));
    if (title) {
      const current = node('span', '', title);
      current.setAttribute('aria-current', 'page');
      items.push(current);
    } else if (column) {
      const current = items.pop();
      current.removeAttribute('href');
      current.setAttribute('aria-current', 'page');
      items.push(current);
    }
    breadcrumb.replaceChildren(...items.map((item) => {
      const li = node('li');
      li.append(item);
      return li;
    }));
  };
  const dateText = (raw) => typeof raw === 'string' ? raw.slice(0, 10) : '';
  const pageURL = (column, page, mode) => {
    const url = new URL('member.html', location.href);
    url.searchParams.set('column', column);
    if (mode === 'video') url.searchParams.set('mode', 'video');
    if (page > 1) url.searchParams.set('page', String(page));
    return url.href;
  };

  const renderPagination = (slot, total, page, size, column, mode) => {
    const count = Math.max(1, Math.ceil(total / size));
    slot.replaceChildren(node('span', '', `共 ${total} 项数据`));
    if (count > 1) {
      const nav = node('nav', 'pagination');
      nav.setAttribute('aria-label', '分页');
      const add = (label, target, disabled, active) => {
        const a = link(label, pageURL(column, target, mode), active ? 'active' : 'page-number');
        if (disabled) { a.removeAttribute('href'); a.classList.add('disabled'); }
        if (active) a.setAttribute('aria-current', 'page');
        nav.append(a);
      };
      add('‹', page - 1, page <= 1, false);
      for (let n = 1; n <= count; n += 1) {
        if (n === 1 || n === count || Math.abs(n - page) <= 2) add(String(n), n, false, n === page);
        else if (n === 2 || n === count - 1) nav.append(node('span', 'ellipsis', '…'));
      }
      add('›', page + 1, page >= count, false);
      slot.append(nav);
    }
    slot.hidden = false;
  };

  const renderStatistics = async () => {
    const slot = shell.querySelector('[data-member-statistics]');
    const state = { year: '', region: '', province: '', subField: '', isBelt: '', isAxis: '' };
    let requestNumber = 0;
    const money = (wan) => (Number(wan || 0) / 10000).toLocaleString('zh-CN', { maximumFractionDigits: 1 });
    const percentage = (value) => value === '' || value == null ? '—' : `${Number(value).toFixed(1)}%`;
    const filter = (label, key, choices, value) => {
      const wrapper = node('label', 'statistics-filter');
      wrapper.append(node('span', '', label));
      const select = node('select');
      select.setAttribute('aria-label', label);
      const values = key === 'year' ? choices.map(String) : ['', ...choices];
      values.forEach((choice) => {
        const flag = key === 'isBelt' || key === 'isAxis';
        const option = node('option', '', choice === '' ? '全部' : flag ? (choice === '1' ? '是' : '否') : choice);
        option.value = choice;
        if (choice === value) option.selected = true;
        select.append(option);
      });
      select.addEventListener('change', () => {
        state[key] = select.value;
        if (key === 'year') state.region = state.province = state.subField = state.isBelt = state.isAxis = '';
        load();
      });
      wrapper.append(select);
      return wrapper;
    };
    const chart = (title, items, metric) => {
      const panel = node('section', 'statistics-chart');
      panel.append(node('h3', '', title));
      if (!items.length) { panel.append(node('p', 'statistics-empty', '当前筛选下暂无数据')); return panel; }
      const ranked = [...items].sort((a, b) => metric === 'count'
        ? Number(b.enterpriseCount) - Number(a.enterpriseCount) : Number(b.revenueYearWan) - Number(a.revenueYearWan));
      const max = Math.max(...ranked.map((item) => metric === 'count' ? Number(item.enterpriseCount) : Number(item.revenueYearWan)), 1);
      const plot = node('div', metric === 'count' ? 'statistics-horizontal' : 'statistics-vertical');
      ranked.forEach((item) => {
        const value = metric === 'count' ? Number(item.enterpriseCount) : Number(item.revenueYearWan);
        const row = node('div', 'statistics-bar-item');
        const bar = node('span', 'statistics-bar');
        row.style.setProperty('--bar-size', `${Math.max(2, value / max * 100)}%`);
        bar.title = `${item.name}：${metric === 'count' ? `${item.enterpriseCount} 家` : `${money(item.revenueYearWan)} 亿元`}${metric === 'growth' ? `；同比 ${percentage(item.yoyPercent)}` : ''}`;
        row.append(node('span', 'statistics-bar-name', item.name), bar,
          node('span', 'statistics-bar-value', metric === 'count' ? item.enterpriseCount : money(item.revenueYearWan)));
        if (metric === 'growth') row.append(node('span', `statistics-yoy ${Number(item.yoyPercent) < 0 ? 'negative' : ''}`, percentage(item.yoyPercent)));
        plot.append(row);
      });
      panel.append(plot);
      if (metric !== 'count') panel.append(node('p', 'statistics-unit', '柱：主营业务收入（亿元）' + (metric === 'growth' ? '　标签：同比增速' : '')));
      return panel;
    };
    const load = async () => {
      const current = ++requestNumber;
      setStatus('正在加载统计数据…');
      const query = new URLSearchParams();
      Object.entries(state).forEach(([key, value]) => { if (value) query.set(key, value); });
      let data;
      try { data = await api(`${apiBase}/statistics/dashboard?${query}`); }
      catch (error) { if (current === requestNumber) handleError(error, '统计数据加载失败，请稍后重试。'); return; }
      if (current !== requestNumber) return;
      if (!data?.statisticsYear) {
        slot.hidden = true;
        setStatus('数据中心暂无已发布的年度统计数据。');
        return;
      }
      state.year = String(data.statisticsYear);
      const controls = node('div', 'statistics-controls');
      controls.append(
        filter('统计年份', 'year', data.years || [], state.year),
        filter('地区范围', 'region', data.filters?.regions || [], state.region),
        filter('省份', 'province', data.filters?.provinces || [], state.province),
        filter('细分领域', 'subField', data.filters?.subFields || [], state.subField),
        filter('一带', 'isBelt', ['1', '0'], state.isBelt),
        filter('一轴', 'isAxis', ['1', '0'], state.isAxis),
      );
      const intro = node('div', 'statistics-intro');
      intro.append(node('p', '', `${data.statisticsYear} 年统计数据 · 对比 ${data.comparisonYear} 年`),
        node('span', '', `数据导入日期：${data.importedAt || '—'}`));
      const summary = node('div', 'statistics-kpis');
      [
        ['覆盖省份', data.summary.provinceCount, '个'],
        ['样本企业数', data.summary.enterpriseCount, '家'],
        ['主营业务收入', money(data.summary.revenueYearWan), '亿元'],
        ['同比增速', percentage(data.summary.totalYoyPercent), ''],
      ].forEach(([label, value, unit]) => {
        const card = node('div', 'statistics-kpi');
        card.append(node('span', '', label), node('strong', '', value), node('small', '', unit));
        summary.append(card);
      });
      const charts = node('div', 'statistics-charts');
      charts.append(
        chart('各省市主营业务收入及同比增速', data.provinces || [], 'growth'),
        chart('各省市企业数量', data.provinces || [], 'count'),
        chart('细分领域主营业务收入', data.subFields || [], 'revenue'),
        chart('各地区主营业务收入及同比增速', data.regions || [], 'growth'),
      );
      const note = node('p', 'statistics-note',
        `收入按原表“万元”汇总后换算为亿元；同比按两年收入合计计算。原表 ${data.summary.recordCount} 条记录，企业名称去重后 ${data.summary.enterpriseCount} 家。`);
      slot.replaceChildren(intro, controls, summary, charts, note);
      slot.hidden = false;
      status.hidden = true;
    };
    await load();
  };

  const renderList = async () => {
    const sidebar = shell.querySelector('[data-member-columns]');
    const renderSidebar = (items = []) => {
      sidebar.replaceChildren(...items, memberSpaceItem());
      sidebar.parentElement.hidden = false;
      shell.classList.add('member-has-columns');
    };
    renderSidebar();
    if (!(await begin())) return;
    let columns;
    try {
      const data = await api(`${apiBase}/member-columns/options`);
      columns = Array.isArray(data?.list) ? data.list.filter((item) => Number(item.status) === 1) : [];
    } catch (error) { handleError(error, '会员栏目加载失败，请稍后重试。'); return; }
    if (!columns.length) { setStatus('暂无可用的会员栏目。'); return; }
    const requested = params.get('column');
    const selected = columns.find((item) => item.name === requested || String(item.id) === requested)
      || columns.find((item) => item.name === '行业报告') || columns[0];
    const column = selected.name;
    const mode = params.get('mode') === 'video' ? 'video' : '';
    renderSidebar(columns.filter((item) => item.name !== '我的空间').map((item) => {
      const li = node('li', item.id === selected.id ? 'is-active' : '');
      const label = item.name;
      const a = link(label, pageURL(label, 1, item.name === '会员专享' ? 'video' : ''));
      if (item.id === selected.id) a.setAttribute('aria-current', 'page');
      li.append(a);
      return li;
    }));
    shell.querySelector('[data-member-heading]').textContent = column;
    setBreadcrumb(column);
    if (selected.name === '数据中心') {
      await renderStatistics();
      return;
    }
    const requestedPage = Number.parseInt(params.get('page') || '1', 10);
    const page = Number.isSafeInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1;
    const size = 10;
    let data;
    try {
      data = await api(`${apiBase}/member-contents/column/${encodeURIComponent(String(selected.id))}?page=${page}&pageSize=${size}`);
    } catch (error) { handleError(error, '会员内容加载失败，请稍后重试。'); return; }
    const total = Number(data?.total) || 0;
    if (total > 0 && page > Math.ceil(total / size)) {
      location.replace(pageURL(column, Math.ceil(total / size), mode));
      return;
    }
    const items = Array.isArray(data?.list) ? data.list : [];
    const slot = shell.querySelector('[data-member-items]');
    slot.replaceChildren(...items.map((item) => {
      const li = node('li', 'news-row member-row');
      const title = link(item.title || '未命名内容', `member-detail.html?id=${encodeURIComponent(item.id)}&column=${encodeURIComponent(column)}`);
      const meta = node('span', 'member-row-meta');
      meta.append(node('span', 'member-type', typeNames[item.type] || '内容'));
      const time = node('time', '', dateText(item.publishTime));
      if (item.publishTime) time.dateTime = dateText(item.publishTime);
      meta.append(time);
      li.append(title, meta);
      return li;
    }));
    slot.hidden = false;
    setStatus(items.length ? '' : '该栏目暂无已发布内容。');
    status.hidden = items.length > 0;
    renderPagination(shell.querySelector('[data-member-pagination]'), total, page, size, column, mode);
  };

  const privateFileName = (raw) => {
    if (typeof raw !== 'string' || !raw) return '';
    try {
      const url = new URL(raw, location.origin);
      if (url.origin !== location.origin || !url.pathname.startsWith(filePrefix) || url.search || url.hash) return '';
      const name = decodeURIComponent(url.pathname.slice(filePrefix.length));
      return /^[A-Za-z0-9_-]{1,80}\.[A-Za-z0-9]{1,10}$/.test(name) ? name : '';
    } catch { return ''; }
  };
  const signedFile = async (raw) => {
    const name = privateFileName(raw);
    if (!name) throw new Error('文件地址无效。');
    const data = await api(`${apiBase}/member-files/sign?name=${encodeURIComponent(name)}`);
    let url;
    try { url = new URL(data?.url, location.origin); } catch { throw new Error('文件签名地址无效。'); }
    if (url.origin !== location.origin || url.pathname !== `${filePrefix}${name}` || !url.searchParams.has('exp') || !url.searchParams.has('sign')) {
      throw new Error('文件签名地址无效。');
    }
    return { url: url.href, expiresIn: Math.min(300, Math.max(1, Number(data.expiresIn) || 300)) };
  };
  const signedImage = async (raw, alt) => {
    const img = node('img');
    img.alt = alt || '';
    try { img.src = (await signedFile(raw)).url; }
    catch { return node('span', 'member-file-error', '图片加载失败'); }
    return img;
  };
  const appendFile = async (slot, raw, label) => {
    if (!raw) return;
    const a = link(label || '查看附件', '#', 'member-action');
    a.addEventListener('click', async (event) => {
      event.preventDefault();
      try { location.href = (await signedFile(raw)).url; }
      catch (error) { setStatus(error.message || '文件签名失败，请稍后重试。'); }
    });
    slot.append(a);
  };

  // Rebuild rich text from an allowlist. No original element or attribute reaches the live DOM.
  const safeRichText = async (raw) => {
    const container = node('div', 'article-body');
    const source = new DOMParser().parseFromString(String(raw || ''), 'text/html');
    const allowed = new Set(['p', 'div', 'span', 'br', 'strong', 'b', 'em', 'i', 'u', 's', 'blockquote', 'ul', 'ol', 'li', 'h2', 'h3', 'h4', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a', 'img']);
    const banned = new Set(['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button', 'svg', 'math', 'video', 'audio', 'source']);
    const pending = [];
    const copy = (from, into) => {
      for (const child of from.childNodes) {
        if (child.nodeType === Node.TEXT_NODE) { into.append(document.createTextNode(child.textContent)); continue; }
        if (child.nodeType !== Node.ELEMENT_NODE) continue;
        const tag = child.localName.toLowerCase();
        if (banned.has(tag)) continue;
        if (!allowed.has(tag)) { copy(child, into); continue; }
        const target = node(tag);
        if (tag === 'img') {
          target.alt = child.getAttribute('alt') || '';
          const file = child.getAttribute('src');
          if (!privateFileName(file)) continue;
          pending.push(signedFile(file).then((signed) => { target.src = signed.url; }).catch(() => { target.replaceWith(node('span', 'member-file-error', '图片加载失败')); }));
        } else if (tag === 'a') {
          const href = child.getAttribute('href') || '';
          if (privateFileName(href)) {
            target.href = '#';
            target.addEventListener('click', async (event) => {
              event.preventDefault();
              try { location.href = (await signedFile(href)).url; }
              catch (error) { setStatus(error.message || '文件签名失败，请稍后重试。'); }
            });
          } else {
            try {
              const url = new URL(href, location.origin);
              if (['http:', 'https:'].includes(url.protocol) && !url.pathname.startsWith(filePrefix)) {
                target.href = url.href;
                target.rel = 'noopener noreferrer';
              }
            } catch { /* Leave unsafe links without href. */ }
          }
        }
        copy(child, target);
        into.append(target);
      }
    };
    copy(source.body, container);
    await Promise.all(pending);
    return container;
  };

  const field = (label, value) => {
    const row = node('div', 'member-data-row');
    row.append(node('dt', '', label), node('dd', '', value == null || value === '' ? '—' : value));
    return row;
  };
  const videoPlayer = (raw, cover) => {
    const stage = node('div', 'member-video-stage');
    const video = node('video');
    video.controls = true;
    video.preload = 'metadata';
    video.playsInline = true;
    const retry = node('button', 'member-action', '重试加载视频');
    retry.type = 'button';
    retry.hidden = true;
    const message = node('p', 'member-video-status', '正在加载视频…');
    stage.append(video, retry, message);
    let timer = 0;
    let refreshing = false;
    let expiry = 0;
    let retries = 0;
    const refresh = async (forcePlay = false) => {
      if (refreshing || !document.contains(video)) return;
      refreshing = true;
      clearTimeout(timer);
      retry.hidden = true;
      message.hidden = false;
      message.textContent = '正在加载视频…';
      const position = Number.isFinite(video.currentTime) ? video.currentTime : 0;
      const shouldPlay = forcePlay || (Boolean(video.currentSrc) && !video.paused);
      try {
        if (!(await verifiedProfile())) {
          const error = new Error('当前账号不是正式会员，无法继续播放。');
          error.code = 403;
          throw error;
        }
        const signed = await signedFile(raw);
        video.src = signed.url;
        video.load();
        await new Promise((resolve, reject) => {
          const finish = (error) => {
            clearTimeout(timeout);
            video.removeEventListener('loadedmetadata', onReady);
            video.removeEventListener('error', onError);
            if (error) reject(error); else resolve();
          };
          const onReady = () => finish();
          const onError = () => finish(new Error('视频加载失败，请重试。'));
          const timeout = setTimeout(() => finish(new Error('视频加载超时，请重试。')), 15000);
          video.addEventListener('loadedmetadata', onReady);
          video.addEventListener('error', onError);
        });
        if (position > 0 && Number.isFinite(video.duration)) video.currentTime = Math.min(position, Math.max(0, video.duration - 0.1));
        if (shouldPlay) await video.play();
        expiry = Date.now() + signed.expiresIn * 1000;
        timer = setTimeout(refresh, Math.max(1000, signed.expiresIn * 1000 - 45000));
        message.hidden = true;
        retries = 0;
      } catch (error) {
        if (error.code === 401 || error.code === 403) {
          video.pause();
          video.removeAttribute('src');
          video.load();
          expiry = 0;
          shell.querySelector('[data-member-content]').hidden = true;
          if (error.code === 401) redirectToLogin();
          else setStatus(error.message, { label: '进入会员中心', href: '/business_member/member/dashboard' });
          return;
        }
        message.textContent = error.message || '视频签名失败，请重试。';
        retry.hidden = false;
        if (video.currentSrc && retries < 2) {
          retries += 1;
          setTimeout(() => refresh(shouldPlay), 700);
        }
      } finally { refreshing = false; }
    };
    retry.addEventListener('click', () => refresh());
    video.addEventListener('play', () => {
      if (expiry && Date.now() > expiry - 30000) refresh();
    });
    video.addEventListener('error', () => {
      if (!refreshing && video.currentSrc && retries < 2) { retries += 1; setTimeout(() => refresh(true), 700); }
      else if (!refreshing) { message.textContent = '视频播放中断，请重试。'; message.hidden = false; retry.hidden = false; }
    });
    if (cover) signedFile(cover).then((signed) => { video.poster = signed.url; }).catch(() => {});
    queueMicrotask(() => refresh());
    return stage;
  };

  const renderDetail = async () => {
    if (!(await begin())) return;
    const id = params.get('id') || '';
    if (!/^[1-9][0-9]*$/.test(id)) { setStatus('内容ID无效或内容不存在。'); return; }
    let item;
    try { item = await api(`${apiBase}/member-contents/detail/${encodeURIComponent(id)}`); }
    catch (error) {
      if (error.code === 1) setStatus('内容不存在、已下线，或暂时无法加载。');
      else handleError(error, '内容加载失败，请稍后重试。');
      return;
    }
    if (!item || !item.title || !typeNames[item.type]) { setStatus('内容不存在或已下线。'); return; }
    const column = columnLabel(item.memberColumnName || params.get('column') || '会员栏目');
    setBreadcrumb(column, item.title);
    document.title = `${item.title} - 中国环保机械行业协会`;
    const slot = shell.querySelector('[data-member-content]');
    const heading = node('header', 'article-header');
    heading.append(node('h1', '', item.title), node('p', 'article-meta', `${typeNames[item.type]}　${dateText(item.publishTime)}　${item.source || ''}`));
    slot.append(heading);
    if (item.type === 1) {
      slot.append(await safeRichText(item.content));
      if (item.attachmentUrl) {
        const files = node('section', 'member-files');
        files.append(node('h2', '', '相关附件'));
        await appendFile(files, item.attachmentUrl, item.attachmentName || '下载附件');
        slot.append(files);
      }
    } else if (item.type === 2) {
      const fields = node('dl', 'member-data');
      [['年份', item.dataYear], ['单位', item.unitName], ['省份', item.province], ['地区', item.region], ['一带', Number(item.isBelt) === 1 ? '是' : '否'], ['一轴', Number(item.isAxis) === 1 ? '是' : '否'], ['细分领域', item.subField], ['主营业务收入（亿元）', item.mainBusinessIncome]].forEach(([label, value]) => fields.append(field(label, value)));
      slot.append(fields);
    } else if (item.type === 3) {
      if (item.cover) slot.append(await signedImage(item.cover, item.title));
      if (item.fullVideoUrl) slot.append(videoPlayer(item.fullVideoUrl, item.cover));
      else slot.append(node('p', 'member-file-error', '视频文件暂不可用。'));
    } else if (item.type === 4) {
      if (item.cover) slot.append(await signedImage(item.cover, item.title));
      if (item.summary) slot.append(node('p', 'member-summary', item.summary));
      const info = node('dl', 'member-data');
      info.append(field('期号', item.issueNo), field('出版年月', item.publishYearMonth));
      slot.append(info);
      if (item.paperFileUrl) {
        const files = node('section', 'member-files');
        await appendFile(files, item.paperFileUrl, item.paperFileName || '查看报刊');
        slot.append(files);
      }
    }
    status.hidden = true;
    slot.hidden = false;
  };

  if (listShell) renderList().catch((error) => handleError(error, '会员内容加载失败，请稍后重试。'));
  else renderDetail().catch((error) => handleError(error, '会员内容加载失败，请稍后重试。'));
})();
