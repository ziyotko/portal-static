(() => {
  const body = document.body;
  const rootPrefix = body.dataset.rootPrefix || './';
  let columnPaths = {};
  try {
    columnPaths = JSON.parse(document.querySelector('#column-paths')?.textContent || '{}');
  } catch {
    columnPaths = {};
  }
  let columnPathHrefs = {};
  try {
    columnPathHrefs = JSON.parse(document.querySelector('#column-path-hrefs')?.textContent || '{}');
  } catch {
    columnPathHrefs = {};
  }

  const breadcrumbItem = (label, href, current = false) => {
    const item = document.createElement('li');
    const content = document.createElement(current || !href ? 'span' : 'a');
    if (current) content.setAttribute('aria-current', 'page');
    if (!current && href) content.href = `${rootPrefix}${href}`;
    content.textContent = label;
    item.append(content);
    return item;
  };

  const elementLabel = (element) => {
    if (!element) return '';
    const copy = element.cloneNode(true);
    copy.querySelectorAll('span').forEach((icon) => icon.remove());
    return copy.textContent.trim();
  };

  const updateDetailLinkContexts = (columnCode) => {
    document.querySelectorAll('[data-detail-link]').forEach((link) => {
      const code = columnCode || link.dataset.defaultColumnCode;
      if (link.dataset.detailBase && code) link.href = `${link.dataset.detailBase}?from=${encodeURIComponent(code)}`;
    });
  };
  const menuButton = document.querySelector('.mobile-menu-button');
  menuButton?.addEventListener('click', () => {
    const open = body.classList.toggle('menu-open');
    menuButton.setAttribute('aria-expanded', String(open));
  });

  document.addEventListener('click', (event) => {
    const button = event.target.closest('.side-menu button[aria-expanded]');
    if (!button) return;
    const expanded = button.getAttribute('aria-expanded') !== 'true';
    button.setAttribute('aria-expanded', String(expanded));
    const submenu = document.getElementById(button.getAttribute('aria-controls'));
    if (submenu) submenu.hidden = !expanded;
  });

  const sectionBreadcrumbNav = document.querySelector('.breadcrumb[data-sync-section]');
  const sectionBreadcrumb = sectionBreadcrumbNav?.querySelector('ol');
  const sectionBreadcrumbBase = sectionBreadcrumb?.innerHTML;
  const defaultSection = document.querySelector('.side-menu > li.is-active > [data-section]');
  const syncSectionBreadcrumb = (sectionItem) => {
    if (!sectionBreadcrumb || !sectionBreadcrumbBase) return;
    sectionBreadcrumb.innerHTML = sectionBreadcrumbBase;
    if (!sectionItem) return;

    const sectionLabel = elementLabel(sectionItem);
    const rootLabel = sectionBreadcrumb.querySelectorAll('li')[1]?.textContent.trim();
    const matchedEntry = Object.entries(columnPaths).find(([, path]) => path[0] === rootLabel && path.at(-1) === sectionLabel);
    const matchedPath = matchedEntry?.[1];
    if (sectionBreadcrumbNav.dataset.syncSection === 'path' && matchedPath) {
      sectionBreadcrumb.replaceChildren();
      const labels = ['首页', ...matchedPath];
      const hrefs = columnPathHrefs[matchedEntry[0]] || [];
      labels.forEach((label, index) => {
        sectionBreadcrumb.append(breadcrumbItem(label, index === 0 ? 'index.html' : hrefs[index - 1], index === labels.length - 1));
      });
      return;
    }

    const current = sectionBreadcrumb.querySelector('[aria-current="page"]');
    if (current) current.textContent = sectionLabel;
  };

  const activateSectionFromHash = () => {
    const section = window.location.hash.slice(1);
    const sectionItems = [...document.querySelectorAll('.side-menu [data-section]')];
    const sectionItem = sectionItems.find((item) => item.dataset.section === section) || defaultSection;
    if (!sectionItem) return;
    document.querySelectorAll('.side-menu > li').forEach((item) => item.classList.remove('is-active'));
    sectionItem.closest('.side-menu > li')?.classList.add('is-active');
    if (sectionItem.matches('button[aria-expanded]')) sectionItem.setAttribute('aria-expanded', 'true');
    syncSectionBreadcrumb(section ? sectionItem : null);
    if (section) {
      const rootLabel = sectionBreadcrumb?.querySelectorAll('li')[1]?.textContent.trim();
      const sectionLabel = elementLabel(sectionItem);
      const matchedCode = Object.entries(columnPaths).find(([, path]) => path[0] === rootLabel && path.at(-1) === sectionLabel)?.[0];
      updateDetailLinkContexts(matchedCode);
    } else {
      updateDetailLinkContexts();
    }
  };
  activateSectionFromHash();
  window.addEventListener('hashchange', activateSectionFromHash);

  const detailPage = document.querySelector('[data-detail-page]');
  if (detailPage) {
    let detailContexts = {};
    try {
      detailContexts = JSON.parse(document.querySelector('#detail-contexts')?.textContent || '{}');
    } catch {
      detailContexts = {};
    }
    const requestedCode = new URLSearchParams(window.location.search).get('from');
    const isHomeSlot = requestedCode === 'home' || requestedCode?.startsWith('home-');
    const columnCode = !isHomeSlot && detailContexts[requestedCode] && columnPaths[requestedCode]
      ? requestedCode : detailPage.dataset.defaultColumnCode;
    const context = detailContexts[columnCode];
    if (context && columnCode !== detailPage.dataset.defaultColumnCode) {
      const grid = detailPage.querySelector('.content-grid');
      const oldSidebar = grid?.querySelector(':scope > aside');
      if (context.sidebar) {
        const fragment = document.createElement('template');
        fragment.innerHTML = context.sidebar;
        const newSidebar = fragment.content.firstElementChild;
        if (oldSidebar) oldSidebar.replaceWith(newSidebar);
        else grid?.prepend(newSidebar);
      } else {
        oldSidebar?.remove();
      }

      const articleCard = grid?.querySelector(':scope > article');
      articleCard?.querySelector('.article-pager')?.remove();
      if (articleCard && (context.previous || context.next)) {
        const pager = document.createElement('nav');
        pager.className = 'article-pager';
        pager.setAttribute('aria-label', '相邻文章');
        for (const [direction, label] of [['previous', '上一篇：'], ['next', '下一篇：']]) {
          const sibling = context[direction];
          if (!sibling) continue;
          const link = document.createElement('a');
          link.className = `article-pager-${direction === 'previous' ? 'prev' : 'next'}`;
          const target = new URL(sibling.href, new URL(rootPrefix, location.href));
          target.searchParams.set('from', columnCode);
          link.href = target.href;
          const strong = document.createElement('strong');
          strong.textContent = label;
          const title = document.createElement('span');
          title.textContent = sibling.title;
          link.append(strong, title);
          pager.append(link);
        }
        articleCard.append(pager);
      }
    }
    const path = columnPaths[columnCode];
    const hrefs = columnPathHrefs[columnCode] || [];
    const breadcrumbList = detailPage.querySelector('[data-detail-breadcrumb] ol');
    if (path?.length && breadcrumbList) {
      breadcrumbList.replaceChildren();
      breadcrumbList.append(breadcrumbItem('首页', 'index.html'));
      path.forEach((label, index) => {
        breadcrumbList.append(breadcrumbItem(label, hrefs[index], index === path.length - 1));
      });

      const menuItems = [...detailPage.querySelectorAll('.side-menu a, .side-menu button')];
      const target = [...path].reverse().map((label) => menuItems.find((item) => elementLabel(item) === label)).find(Boolean);
      if (target) {
        detailPage.querySelectorAll('.side-menu > li').forEach((item) => item.classList.remove('is-active'));
        detailPage.querySelectorAll('.side-menu li.current').forEach((item) => item.classList.remove('current'));
        target.closest('li')?.classList.add('current');
        target.closest('.side-menu > li')?.classList.add('is-active');
        const submenu = target.closest('.side-submenu');
        submenu?.previousElementSibling?.querySelector('button[aria-expanded]')?.setAttribute('aria-expanded', 'true');
        if (submenu) submenu.hidden = false;
      }
    }
  }

  const toast = document.querySelector('.toast');
  let toastTimer;
  const showToast = (message) => {
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.classList.remove('show'), 2200);
  };

  document.querySelectorAll('a[href="#coming-soon"]').forEach((link) => {
    link.addEventListener('click', (event) => {
      event.preventDefault();
      showToast('敬请期待');
    });
  });

  document.querySelectorAll('a[href="#"], a[href="#mobile"]').forEach((link) => {
    link.addEventListener('click', (event) => {
      event.preventDefault();
      showToast(link.hasAttribute('data-demo-link') ? '外部友情链接为静态演示，正式地址待配置' : '该内容为静态交互演示');
    });
  });

  const quickMobile = document.querySelector('.quick-mobile');
  const mobileQrTrigger = quickMobile?.querySelector('.quick-mobile-trigger');
  const mobileQrCard = quickMobile?.querySelector('.mobile-qr-card');
  let mobileQrPinned = false;
  const setMobileQrOpen = (open) => {
    if (!mobileQrTrigger || !mobileQrCard) return;
    mobileQrTrigger.setAttribute('aria-expanded', String(open));
    mobileQrCard.hidden = !open;
  };
  quickMobile?.addEventListener('mouseenter', () => setMobileQrOpen(true));
  quickMobile?.addEventListener('mouseleave', () => {
    if (!mobileQrPinned) setMobileQrOpen(false);
  });
  quickMobile?.addEventListener('focusin', () => setMobileQrOpen(true));
  quickMobile?.addEventListener('focusout', (event) => {
    if (!mobileQrPinned && !quickMobile.contains(event.relatedTarget)) setMobileQrOpen(false);
  });
  mobileQrTrigger?.addEventListener('click', () => {
    mobileQrPinned = !mobileQrPinned;
    setMobileQrOpen(mobileQrPinned);
  });
  document.addEventListener('click', (event) => {
    if (!quickMobile?.contains(event.target)) {
      mobileQrPinned = false;
      setMobileQrOpen(false);
    }
  });
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape' || mobileQrCard?.hidden) return;
    mobileQrPinned = false;
    setMobileQrOpen(false);
    mobileQrTrigger?.focus();
  });

  document.querySelectorAll('.search-form, .search-page-form').forEach((form) => {
    form.addEventListener('submit', (event) => {
      const value = form.querySelector('input')?.value.trim() || '';
      if (!value) {
        event.preventDefault();
        showToast('请输入搜索关键词');
      }
    });
  });

  const searchPage = document.querySelector('[data-search-page]');
  if (searchPage) {
    const params = new URLSearchParams(window.location.search);
    const query = (params.get('q') || '').trim();
    const normalizedQuery = query.toLocaleLowerCase('zh-CN');
    const requestedPage = Number.parseInt(params.get('page') || '1', 10);
    const pageSize = 10;
    const rows = [...searchPage.querySelectorAll('[data-search-result]')];
    const summary = searchPage.querySelector('[data-search-summary]');
    const empty = searchPage.querySelector('[data-search-empty]');
    const pagination = searchPage.querySelector('[data-search-pagination]');
    const pagesSlot = searchPage.querySelector('[data-search-pages]');

    document.querySelectorAll('input[name="q"]').forEach((input) => { input.value = query; });
    const matches = query ? rows.filter((row) => row.textContent.toLocaleLowerCase('zh-CN').includes(normalizedQuery)) : [];
    const pageCount = Math.max(1, Math.ceil(matches.length / pageSize));
    const currentPage = Math.min(Math.max(Number.isFinite(requestedPage) ? requestedPage : 1, 1), pageCount);
    const visible = new Set(matches.slice((currentPage - 1) * pageSize, currentPage * pageSize));
    rows.forEach((row) => { row.hidden = !visible.has(row); });

    if (summary) summary.textContent = query ? `共找到 ${matches.length} 条与“${query}”相关的内容` : '请输入关键词进行搜索';
    if (empty) empty.hidden = !query || matches.length > 0;
    if (pagination) pagination.hidden = matches.length <= pageSize;

    const goToPage = (page) => {
      const next = new URL(window.location.href);
      next.searchParams.set('q', query);
      next.searchParams.set('page', String(page));
      window.location.href = next.href;
    };

    if (pagesSlot && matches.length > pageSize) {
      pagesSlot.replaceChildren();
      const visiblePages = [];
      for (let page = 1; page <= pageCount; page += 1) {
        if (page === 1 || page === pageCount || (page >= currentPage - 2 && page <= currentPage + 2)) {
          visiblePages.push(page);
        }
      }
      let previousPage = 0;
      visiblePages.forEach((page) => {
        if (previousPage && page > previousPage + 1) {
          const ellipsis = document.createElement('span');
          ellipsis.className = 'ellipsis';
          ellipsis.setAttribute('aria-hidden', 'true');
          ellipsis.textContent = '…';
          pagesSlot.append(ellipsis);
        }
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'page-number';
        button.textContent = String(page);
        button.classList.toggle('active', page === currentPage);
        if (page === currentPage) button.setAttribute('aria-current', 'page');
        button.addEventListener('click', () => goToPage(page));
        pagesSlot.append(button);
        previousPage = page;
      });
    }
    const previousButton = searchPage.querySelector('[data-search-prev]');
    const nextButton = searchPage.querySelector('[data-search-next]');
    if (previousButton) {
      previousButton.disabled = currentPage === 1;
      previousButton.addEventListener('click', () => goToPage(currentPage - 1));
    }
    if (nextButton) {
      nextButton.disabled = currentPage === pageCount;
      nextButton.addEventListener('click', () => goToPage(currentPage + 1));
    }
  }

  const wireTabs = (selector) => {
    document.querySelectorAll(selector).forEach((tabList) => {
      const buttons = [...tabList.querySelectorAll('[data-tab]')];
      const activate = (button) => {
        const panelId = button.dataset.tab;
        buttons.forEach((item) => item.classList.toggle('active', item === button));
        document.querySelectorAll(`[data-tab-group="${tabList.dataset.group}"]`).forEach((panel) => {
          panel.hidden = panel.id !== panelId;
        });
      };
      buttons.forEach((button) => {
        button.addEventListener('mouseenter', () => activate(button));
        button.addEventListener('focus', () => activate(button));
        button.addEventListener('click', () => {
          activate(button);
          if (button.dataset.href) window.location.href = button.dataset.href;
        });
      });
    });
  };
  wireTabs('.home-tabs');
  wireTabs('.updates-tabs');

  document.querySelectorAll('.hero-slider').forEach((slider) => {
    const slides = [...slider.querySelectorAll('.hero-slide')];
    const dots = [...slider.querySelectorAll('[data-slide-to]')];
    if (slides.length < 2) return;

    let activeIndex = Math.max(0, slides.findIndex((slide) => slide.classList.contains('active')));
    let timer;
    const showSlide = (index) => {
      activeIndex = (index + slides.length) % slides.length;
      slides.forEach((slide, slideIndex) => {
        const isActive = slideIndex === activeIndex;
        slide.classList.toggle('active', isActive);
        slide.setAttribute('aria-hidden', String(!isActive));
        if (isActive) slide.removeAttribute('tabindex');
        else slide.setAttribute('tabindex', '-1');
      });
      dots.forEach((dot, dotIndex) => {
        const isActive = dotIndex === activeIndex;
        dot.classList.toggle('active', isActive);
        if (isActive) dot.setAttribute('aria-current', 'true');
        else dot.removeAttribute('aria-current');
      });
    };
    const stop = () => clearInterval(timer);
    const start = () => {
      stop();
      if (!document.hidden) timer = setInterval(() => showSlide(activeIndex + 1), 5000);
    };

    dots.forEach((dot) => dot.addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      showSlide(Number(dot.dataset.slideTo));
      start();
    }));
    slider.addEventListener('mouseenter', stop);
    slider.addEventListener('mouseleave', start);
    slider.addEventListener('focusin', stop);
    slider.addEventListener('focusout', (event) => {
      if (!slider.contains(event.relatedTarget)) start();
    });
    document.addEventListener('visibilitychange', () => document.hidden ? stop() : start());
    showSlide(activeIndex);
    start();
  });

  document.querySelectorAll('.pagination').forEach((pagination) => {
    pagination.addEventListener('click', (event) => {
      const button = event.target.closest('button[data-page]');
      if (!button) return;
      pagination.querySelectorAll('button').forEach((item) => item.classList.remove('active'));
      button.classList.add('active');
      const page = Number(button.dataset.page);
      document.querySelectorAll('.news-row a').forEach((link, index) => {
        const base = link.dataset.baseTitle || link.textContent.trim();
        link.dataset.baseTitle = base;
        if (page > 1) link.textContent = `${base}（第 ${page} 页示例 ${index + 1}）`;
        else link.textContent = base;
      });
      showToast(`已切换到第 ${page} 页（静态演示）`);
    });
  });

  document.querySelectorAll('.page-jump input[data-page-prefix]').forEach((input) => {
    const navigate = () => {
      const page = Math.min(Number(input.max) || 1, Math.max(Number(input.min) || 1, Number(input.value) || 1));
      window.location.href = `${input.dataset.pagePrefix}${page}.html`;
    };
    input.addEventListener('change', navigate);
    input.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') navigate();
    });
  });

  document.querySelectorAll('[data-inline-video-play]').forEach((button) => {
    const video = button.closest('.video-stage')?.querySelector('video');
    if (!video) return;
    button.addEventListener('click', () => video.play().catch(() => {}));
    video.addEventListener('play', () => { button.hidden = true; });
    video.addEventListener('ended', () => { button.hidden = false; });
  });

  const backToTop = document.querySelector('.back-to-top');
  window.addEventListener('scroll', () => backToTop?.classList.toggle('visible', window.scrollY > 500), { passive: true });
  backToTop?.addEventListener('click', () => window.scrollTo({ top: 0, behavior: 'smooth' }));
})();
