(() => {
  const body = document.body;
  const rootPrefix = body.dataset.rootPrefix || './';
  let columnPaths = {};
  try {
    columnPaths = JSON.parse(document.querySelector('#column-paths')?.textContent || '{}');
  } catch {
    columnPaths = {};
  }

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

  const quickRail = document.querySelector('.quick-rail');
  if (quickRail) {
    const desktopRail = window.matchMedia('(min-width: 781px)');
    let railDocumentTop = 0;
    let railFrame = 0;

    const syncQuickRail = () => {
      railFrame = 0;
      if (!desktopRail.matches) {
        quickRail.classList.remove('is-following');
        return;
      }
      quickRail.classList.toggle('is-following', window.scrollY + 24 >= railDocumentTop);
    };

    const measureQuickRail = () => {
      quickRail.classList.remove('is-following');
      const rect = quickRail.getBoundingClientRect();
      railDocumentTop = window.scrollY + rect.top;
      quickRail.style.setProperty('--quick-rail-left', `${rect.left}px`);
      quickRail.style.setProperty('--quick-rail-width', `${rect.width}px`);
      syncQuickRail();
    };

    const requestQuickRailSync = () => {
      if (!railFrame) railFrame = window.requestAnimationFrame(syncQuickRail);
    };

    measureQuickRail();
    window.addEventListener('scroll', requestQuickRailSync, { passive: true });
    window.addEventListener('resize', measureQuickRail);
    desktopRail.addEventListener?.('change', measureQuickRail);
  }

  document.querySelectorAll('.side-menu button[aria-expanded]').forEach((button) => {
    button.addEventListener('click', () => {
      button.setAttribute('aria-expanded', String(button.getAttribute('aria-expanded') !== 'true'));
    });
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
    const matchedPath = Object.values(columnPaths).find((path) => path[0] === rootLabel && path.at(-1) === sectionLabel);
    if (sectionBreadcrumbNav.dataset.syncSection === 'path' && matchedPath) {
      sectionBreadcrumb.replaceChildren();
      const labels = ['首页', ...matchedPath];
      labels.forEach((label, index) => {
        const item = document.createElement('li');
        if (index === labels.length - 1) {
          const current = document.createElement('span');
          current.setAttribute('aria-current', 'page');
          current.textContent = label;
          item.append(current);
        } else {
          const link = document.createElement('a');
          link.href = index === 0 ? '../index.html' : '#';
          link.textContent = label;
          item.append(link);
        }
        sectionBreadcrumb.append(item);
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
    const requestedCode = new URLSearchParams(window.location.search).get('from');
    const columnCode = columnPaths[requestedCode] ? requestedCode : detailPage.dataset.defaultColumnCode;
    const path = columnPaths[columnCode];
    const detailTitle = detailPage.querySelector('.article-header h1')?.textContent.trim();
    const breadcrumbList = detailPage.querySelector('[data-detail-breadcrumb] ol');
    if (path?.length && detailTitle && breadcrumbList) {
      breadcrumbList.replaceChildren();
      const homeItem = document.createElement('li');
      const homeLink = document.createElement('a');
      homeLink.href = `${rootPrefix}index.html`;
      homeLink.textContent = '首页';
      homeItem.append(homeLink);
      breadcrumbList.append(homeItem);
      path.forEach((label) => {
        const item = document.createElement('li');
        const link = document.createElement('a');
        link.href = '#';
        link.textContent = label;
        item.append(link);
        breadcrumbList.append(item);
      });
      const titleItem = document.createElement('li');
      const current = document.createElement('span');
      current.setAttribute('aria-current', 'page');
      current.textContent = detailTitle;
      titleItem.append(current);
      breadcrumbList.append(titleItem);

      const menuItems = [...detailPage.querySelectorAll('.side-menu a, .side-menu button')];
      const target = [...path].reverse().map((label) => menuItems.find((item) => elementLabel(item) === label)).find(Boolean);
      if (target) {
        detailPage.querySelectorAll('.side-menu > li').forEach((item) => item.classList.remove('is-active'));
        detailPage.querySelectorAll('.side-menu li.current').forEach((item) => item.classList.remove('current'));
        target.closest('li')?.classList.add('current');
        target.closest('.side-menu > li')?.classList.add('is-active');
        target.closest('.side-submenu')?.previousElementSibling?.setAttribute('aria-expanded', 'true');
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
    const pageSize = 6;
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
      for (let page = 1; page <= pageCount; page += 1) {
        const button = document.createElement('button');
        button.type = 'button';
        button.textContent = String(page);
        button.classList.toggle('active', page === currentPage);
        if (page === currentPage) button.setAttribute('aria-current', 'page');
        button.addEventListener('click', () => goToPage(page));
        pagesSlot.append(button);
      }
    }
    searchPage.querySelector('[data-search-prev]')?.addEventListener('click', () => goToPage(Math.max(1, currentPage - 1)));
    searchPage.querySelector('[data-search-next]')?.addEventListener('click', () => goToPage(Math.min(pageCount, currentPage + 1)));
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

    dots.forEach((dot) => dot.addEventListener('click', () => {
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

  document.querySelectorAll('.language-switch').forEach((button) => {
    button.addEventListener('click', () => showToast('英文版内容将在后续阶段接入'));
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
