(function () {
  const page = document.body.dataset.page || "";
  const header = document.getElementById("site-header");
  const footer = document.getElementById("site-footer");
  const rootPrefix = document.body.dataset.rootPrefix || "./";
  const generatedContent = window.MIIC_STATIC_CONTENT || { importantNews: [], articlePaths: {} };

  function rootUrl(value) {
    if (!value || /^(?:[a-z]+:|\/\/|#)/i.test(value)) return value;
    return rootPrefix + value.replace(/^\.\//, "").replace(/^\//, "");
  }

  const navItems = [
    ["news", "资讯动态", rootUrl("news.html")],
    ["business", "核心业务", rootUrl("business.html")],
    ["platforms", "服务平台", rootUrl("platforms.html")],
    ["about", "关于我们", rootUrl("about.html")]
  ];

  const params = new URLSearchParams(window.location.search);
  const requestedSection = params.get("section");
  const validSections = navItems.map(function (item) { return item[0]; });
  const activeRoot = page === "article" ? (document.body.dataset.activeRoot || "news") : page === "list" ? "news" : page === "detail" && validSections.includes(requestedSection)
    ? requestedSection
    : page === "platform-detail" ? "platforms"
      : page === "service-detail" ? "business" : page;

  header.innerHTML = `
    <div class="nav-shell shell">
      <a class="brand" href="${rootUrl("news.html")}" aria-label="机械工业信息中心资讯动态">
        <img class="brand-logo" src="${rootUrl("assets/miic-logo.png")}" alt="机械工业信息中心">
      </a>
      <button class="menu-toggle" id="menu-toggle" type="button" aria-label="打开导航" aria-expanded="false">
        <i></i><i></i><i></i>
      </button>
      <nav class="main-nav" id="main-nav" aria-label="主导航">
        ${navItems.map(([key, label, href]) => `
          <a class="${activeRoot === key ? "is-active" : ""}" href="${href}">${label}</a>
        `).join("")}
      </nav>
    </div>
  `;

  footer.innerHTML = `
    <div class="footer-main shell">
      <div class="footer-intro">
        <a class="footer-brand" href="${rootUrl("news.html")}"><img src="${rootUrl("assets/miic-logo.png")}" alt="机械工业信息中心"></a>
        <p>立足行业公共服务职能，服务机械工业高质量发展。</p>
      </div>
      <div class="footer-contact">
        <strong>联系方式</strong>
        <a href="tel:01068510153">业务咨询：010-68510153</a>
        <a href="tel:01068510148">客户服务：010-68510148</a>
      </div>
      <div>
        <strong class="footer-column-title">网站导航</strong>
        <div class="footer-nav">
          ${navItems.map(([, label, href]) => `<a href="${href}">${label}</a>`).join("")}
        </div>
      </div>
    </div>
    <div class="footer-bottom">
      <div class="shell">
        <span>© 2026 机械工业信息中心</span>
        <div class="footer-filings">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">京ICP备06006308号-2</a>
          <a class="footer-police-filing" href="https://beian.mps.gov.cn/#/query/webSearch?code=11010202010987" target="_blank" rel="noopener">
            <img src="${rootUrl("assets/gongan-beian.png")}" alt="公安备案图标" width="20" height="20">
            <span>京公网安备11010202010987号</span>
          </a>
        </div>
      </div>
    </div>
  `;

  const menuToggle = document.getElementById("menu-toggle");
  const mainNav = document.getElementById("main-nav");
  menuToggle.addEventListener("click", function () {
    const isOpen = mainNav.classList.toggle("is-open");
    menuToggle.setAttribute("aria-expanded", String(isOpen));
    menuToggle.setAttribute("aria-label", isOpen ? "关闭导航" : "打开导航");
  });

  const importantCarousel = document.querySelector("[data-important-carousel]");

  if (importantCarousel) {
    const fallbackImportantNews = [
      {
        category: "中心动态",
        date: "2026.07.18",
        title: "聚焦行业数智化转型，构建机械工业高质量发展新动能",
        summary: "围绕数据价值释放、转型能力提升和行业共性需求，持续完善数智化赋能服务体系。",
        image: "./assets/unsplash-data-dashboard.jpg",
        imageAlt: "数字化运营数据看板",
        href: "./detail.html?section=news&id=news-1"
      },
      {
        category: "行业动态",
        date: "2026.07.11",
        title: "机械工业运行总体平稳，产业结构持续优化升级",
        summary: "跟踪行业运行态势与重点领域变化，为产业分析与科学决策提供专业信息参考。",
        image: "./assets/unsplash-gears.jpg",
        imageAlt: "机械工业齿轮部件",
        href: "./detail.html?section=news&id=news-2"
      },
      {
        category: "成果发布",
        date: "2026.07.03",
        title: "机械工业数字化转型路径与实践研究成果发布",
        summary: "总结典型实践，提炼转型方法，为行业企业数字化建设提供可复用的经验参考。",
        image: "./assets/unsplash-precision-lab.jpg",
        imageAlt: "精密测量设备检测机械部件",
        href: "./detail.html?section=news&id=news-4"
      }
    ];
    const importantNews = generatedContent.generated === true
      ? (Array.isArray(generatedContent.importantNews) ? generatedContent.importantNews.slice() : [])
      : fallbackImportantNews;
    if (!importantNews.length) {
      importantNews.push({
        category: "资讯动态",
        date: "",
        title: "暂无中心重要动态",
        summary: "后台发布内容后，将在下一次静态化时自动更新。",
        image: "./assets/miic-placeholder.png",
        imageAlt: "暂无中心重要动态",
        href: "./news.html"
      });
    }

    importantCarousel.innerHTML = `
      <div class="important-carousel__track">
        ${importantNews.map(function (item, index) {
          return `
            <article class="important-carousel__slide${index === 0 ? " is-active" : ""}" role="group" aria-roledescription="幻灯片" aria-label="${index + 1} / ${importantNews.length}" aria-hidden="${index === 0 ? "false" : "true"}">
              <img src="${rootUrl(item.image)}" alt="${item.imageAlt}"${index === 0 ? " fetchpriority=\"high\"" : " loading=\"lazy\""}>
              <div class="important-carousel__shade"></div>
              <div class="shell important-carousel__content">
                <p class="important-carousel__meta"><span>中心重要动态</span><b>${item.category}</b><time>${item.date}</time></p>
                <h2>${item.title}</h2>
                <p class="important-carousel__summary">${item.summary}</p>
                <a href="${rootUrl(item.href)}"${item.external ? " target=\"_blank\" rel=\"noopener noreferrer\"" : ""}>查看详情 <span aria-hidden="true">→</span></a>
              </div>
            </article>
          `;
        }).join("")}
      </div>
      <div class="shell important-carousel__controls">
        <div class="important-carousel__dots" role="tablist" aria-label="选择重要动态">
          ${importantNews.map(function (item, index) {
            return `<button type="button" role="tab" aria-label="查看第 ${index + 1} 条：${item.title}" aria-selected="${index === 0 ? "true" : "false"}" data-carousel-index="${index}"></button>`;
          }).join("")}
        </div>
      </div>
    `;

    const carouselSlides = Array.from(importantCarousel.querySelectorAll(".important-carousel__slide"));
    const carouselDots = Array.from(importantCarousel.querySelectorAll("[data-carousel-index]"));
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    let activeSlide = 0;
    let carouselTimer;

    function showSlide(index) {
      activeSlide = (index + carouselSlides.length) % carouselSlides.length;
      carouselSlides.forEach(function (slide, slideIndex) {
        const isActive = slideIndex === activeSlide;
        slide.classList.toggle("is-active", isActive);
        slide.setAttribute("aria-hidden", String(!isActive));
      });
      carouselDots.forEach(function (dot, dotIndex) {
        dot.setAttribute("aria-selected", String(dotIndex === activeSlide));
      });
    }

    function stopCarousel() {
      window.clearInterval(carouselTimer);
    }

    function startCarousel() {
      stopCarousel();
      if (!reduceMotion && !document.hidden && !importantCarousel.matches(":hover") && !importantCarousel.contains(document.activeElement)) {
        carouselTimer = window.setInterval(function () {
          showSlide(activeSlide + 1);
        }, 5000);
      }
    }

    carouselDots.forEach(function (dot) {
      dot.addEventListener("click", function () {
        showSlide(Number(dot.dataset.carouselIndex));
        startCarousel();
      });
    });

    importantCarousel.addEventListener("keydown", function (event) {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        showSlide(activeSlide + (event.key === "ArrowLeft" ? -1 : 1));
        startCarousel();
      }
    });
    importantCarousel.addEventListener("mouseenter", stopCarousel);
    importantCarousel.addEventListener("mouseleave", startCarousel);
    importantCarousel.addEventListener("focusin", stopCarousel);
    importantCarousel.addEventListener("focusout", startCarousel);
    document.addEventListener("visibilitychange", startCarousel);
    startCarousel();
  }

  const detailPages = {
    "news-1": {
      section: "news",
      label: "中心动态",
      title: "聚焦行业数智化转型，构建机械工业高质量发展新动能",
      lead: "围绕数据价值释放、转型能力提升和行业共性需求，持续完善数智化赋能服务体系。",
      meta: "发布日期：2026年7月18日",
      image: "./assets/unsplash-data-dashboard.jpg",
      imageAlt: "数字化运营数据看板",
      blocks: [
        ["工作重点", "机械工业信息中心持续加强行业数字化转型研究和专业服务能力建设，围绕行业数据、技术应用和企业转型需求，完善多层次服务体系。"],
        ["服务方向", "以行业共性需求为牵引，推进信息技术服务、数据要素服务、评价咨询和能力培育协同发展，为行业高质量发展提供专业支撑。"]
      ]
    },
    "news-2": {
      section: "news",
      label: "行业动态",
      title: "机械工业运行总体平稳，产业结构持续优化升级",
      lead: "跟踪行业运行态势与重点领域变化，为产业分析与科学决策提供专业信息参考。",
      meta: "发布日期：2026年7月11日",
      image: "./assets/unsplash-gears.jpg",
      imageAlt: "机械工业齿轮部件",
      blocks: [
        ["运行态势", "机械工业主要指标保持平稳，重点领域发展韧性持续显现，产业结构进一步向高端化、智能化和绿色化方向调整。"],
        ["趋势观察", "技术创新、设备更新和数字化协同成为推动行业转型升级的重要力量，企业对专业信息和决策支持的需求持续增长。"]
      ]
    },
    "news-3": {
      section: "news",
      label: "行业资讯",
      title: "制造业数字化转型加速向全链条协同纵深推进",
      lead: "从单点技术应用走向流程重构与数据协同，数智化正在形成产业发展新优势。",
      meta: "发布日期：2026年7月8日",
      image: "./assets/unsplash-appliance-line.jpg",
      imageAlt: "自动化制造生产线",
      blocks: [
        ["转型变化", "制造企业数字化建设正在从单一系统部署转向研发、生产、供应链和服务环节的协同优化。"],
        ["应用方向", "工业软件、智能装备、数据治理和人工智能等能力加快融合，推动企业提升运营效率和创新能力。"]
      ]
    },
    "news-4": {
      section: "news",
      label: "成果发布",
      title: "机械工业数字化转型路径与实践研究成果发布",
      lead: "总结典型实践，提炼转型方法，为行业企业数字化建设提供可复用的经验参考。",
      meta: "发布日期：2026年7月3日",
      image: "./assets/unsplash-precision-lab.jpg",
      imageAlt: "精密测量设备检测机械部件",
      blocks: [
        ["成果内容", "研究围绕战略规划、基础能力、业务协同、数据应用和组织保障等方面，系统梳理机械工业企业数字化转型路径。"],
        ["成果价值", "通过总结典型场景和实施经验，为不同基础、不同规模企业制定转型方案提供参考。"]
      ]
    },
    "news-5": {
      section: "news",
      label: "行业资讯",
      title: "数据要素加快融入机械工业研发、生产与服务环节",
      lead: "推动行业数据资源规范治理与安全流通，释放数据在产业协同中的应用价值。",
      meta: "发布日期：2026年6月26日",
      image: "./assets/unsplash-data-analysis.jpg",
      imageAlt: "工业数据指标分析界面",
      blocks: [
        ["应用进展", "数据要素正加快融入产品研发、生产制造、供应链管理、设备运维和客户服务等业务环节。"],
        ["建设重点", "行业需要持续完善数据标准、质量治理、安全保障和流通机制，推动数据资源转化为可复用的产品与服务。"]
      ]
    },
    "news-6": {
      section: "news",
      label: "中心动态",
      title: "中心持续完善行业信息采集与专业内容服务机制",
      lead: "围绕重点领域和企业需求优化信息采集、分析与发布流程，提升行业信息服务质效。",
      meta: "发布日期：2026年6月18日",
      image: "./assets/unsplash-newsroom-2.jpg",
      imageAlt: "团队开展行业信息采集与编辑工作",
      blocks: [
        ["服务机制", "中心持续梳理行业信息需求，完善公开信息采集、内容审核、专题策划和成果发布流程，提升信息服务的及时性与专业性。"],
        ["能力建设", "通过加强重点领域跟踪和专业团队协同，形成覆盖信息汇聚、分析研判与内容输出的服务机制，为行业用户提供可靠参考。"]
      ]
    },
    "news-7": {
      section: "news",
      label: "行业动态",
      title: "智能装备应用场景持续拓展，制造体系加快柔性升级",
      lead: "智能机器人与数字化生产系统加速融合，为制造企业提质增效提供新的技术支撑。",
      meta: "发布日期：2026年6月10日",
      image: "./assets/unsplash-robot-factory.jpg",
      imageAlt: "智能机器人参与制造生产",
      blocks: [
        ["应用趋势", "智能装备正在从单一工序应用向多场景协同延伸，在装配、检测、搬运和仓储等环节形成更加灵活的生产能力。"],
        ["发展重点", "企业需进一步加强装备互联、生产数据治理和人员能力建设，推动智能设备与业务流程深度融合，持续提升制造体系韧性。"]
      ]
    },
  };

  const requestedDetailId = params.get("id") || "";
  const generatedArticlePath = generatedContent.articlePaths && generatedContent.articlePaths[requestedDetailId];
  if (page === "detail" && /^\d+$/.test(requestedDetailId) && generatedArticlePath) {
    window.location.replace(rootUrl(generatedArticlePath));
    return;
  }

  if (page === "detail") {
    const detail = detailPages[params.get("id")] || detailPages["news-1"];
    const sectionItem = navItems.find(function (item) { return item[0] === detail.section; });
    const title = document.getElementById("detail-title");
    const lead = document.getElementById("detail-lead");
    const category = document.getElementById("detail-category");
    const date = document.getElementById("detail-date");
    const back = document.getElementById("detail-back");
    const content = document.getElementById("detail-content");

    document.title = detail.title + " - 机械工业信息中心";
    title.textContent = detail.title;
    lead.textContent = detail.lead;
    category.textContent = detail.label;
    date.textContent = detail.meta;
    back.href = sectionItem[2];
    back.textContent = sectionItem[1];
    content.innerHTML = detail.blocks.map(function (block) {
      return `
        <section>
          <h2>${block[0]}</h2>
          <p>${block[1]}</p>
        </section>
      `;
    }).join("");
  }

  const platformProfiles = {
    saas: {
      label: "服务体系",
      title: "标准化SaaS服务",
      lead: "依托综合性机械行业服务云，为行业组织和企事业单位提供开箱即用、持续迭代的标准化软件服务。",
      image: "./assets/hero-industrial.png",
      imageAlt: "工业软件数字化应用",
      tags: ["门户网站", "协同办公", "会议管理", "会员服务"],
      overview: "平台围绕机械行业机构的共性数字化需求，将门户建设、组织协同、会议活动和会员运营等能力沉淀为标准化产品。通过统一技术底座和模块化配置，降低系统建设与日常维护门槛，支持用户快速形成稳定、安全、可持续运营的数字化服务入口。",
      audience: "行业协会、学会组织、院校、制造企业及科研机构",
      value: "缩短建设周期，降低技术门槛，持续获得产品升级与运营支持",
      capabilities: [
        ["门户网站建设", "提供内容发布、栏目管理、专题页面和多终端适配能力，支持机构快速建立统一品牌窗口。"],
        ["协同办公", "覆盖通知、流程、任务和资料协作等常用场景，提升组织内部沟通与执行效率。"],
        ["会议与活动管理", "支持报名、审核、签到、通知和会后资料管理，形成完整的活动服务流程。"],
        ["会员与问卷服务", "提供会员档案、分级管理、信息触达和在线调研能力，辅助组织开展精细化服务。"]
      ],
      process: ["需求沟通", "模块配置", "部署上线", "持续运营"]
    },
    data: {
      label: "专项平台",
      title: "行业大数据服务平台",
      lead: "面向机械工业多来源数据资源，提供规范汇聚、治理加工、主题分析和数据服务管理能力。",
      image: "./assets/unsplash-data-center.jpg",
      imageAlt: "数据中心服务器与网络设备",
      tags: ["数据汇聚", "标准治理", "主题分析", "数据服务"],
      overview: "平台以行业数据资源的规范管理和价值释放为目标，连接业务系统、公开数据与专题数据资源，建立统一的数据目录、质量规则和主题数据体系。通过可视化分析与服务接口，为行业运行研究、产业观察和业务决策提供可追溯的数据基础。",
      audience: "行业管理机构、研究单位、产业园区及具有数据治理需求的企业",
      value: "提升数据质量与利用效率，为分析研究和管理决策提供统一数据支撑",
      capabilities: [
        ["多源数据归集", "支持结构化与非结构化数据接入，形成可查询、可追溯的行业数据资源目录。"],
        ["数据治理与标准化", "围绕数据标准、质量校验、主数据和标签体系开展规范化治理。"],
        ["主题数据分析", "按产业、区域、企业和产品等主题组织指标，提供趋势与结构分析视图。"],
        ["数据服务管理", "统一管理数据产品、接口权限和使用记录，支持数据能力安全有序复用。"]
      ],
      process: ["资源盘点", "接入治理", "主题建设", "服务运营"]
    },
    opinion: {
      label: "专项平台",
      title: "行业舆情服务平台",
      lead: "围绕机械行业公开信息开展持续汇聚、主题监测、趋势研判与专业信息服务。",
      image: "./assets/unsplash-newsroom-1.jpg",
      imageAlt: "媒体采访与公开信息采集",
      tags: ["信息汇聚", "主题监测", "趋势研判", "专题服务"],
      overview: "平台面向政策、产业、企业和市场等公开信息建立常态化采集与分类机制，通过主题规则、标签体系和趋势分析帮助用户及时了解相关动态。平台强调信息来源可追溯与人工专业研判相结合，为专题研究和信息服务提供基础支持。",
      audience: "行业组织、研究机构、产业管理部门及关注行业动态的企业",
      value: "提高信息获取效率，及时识别重点议题和行业趋势，辅助专题研判",
      capabilities: [
        ["公开信息汇聚", "持续采集政策、媒体、机构与企业公开信息，并保留来源和发布时间。"],
        ["主题信息监测", "根据行业领域、重点企业和专题任务配置监测规则与关注范围。"],
        ["趋势分析展示", "从信息热度、时间变化和主题分布等维度呈现公开信息变化。"],
        ["专题信息输出", "结合专业筛选与分析，形成简报、专题汇编和定向信息服务。"]
      ],
      process: ["监测配置", "信息采集", "分析研判", "成果输出"]
    },
    sme: {
      label: "专项平台",
      title: "行业中小企业服务平台",
      lead: "汇聚政策、公共资源和专业服务，为机械行业中小企业提供普惠、便捷的服务入口。",
      image: "./assets/unsplash-manufacturing-team.jpg",
      imageAlt: "中小制造企业团队生产场景",
      tags: ["政策服务", "资源汇聚", "需求对接", "普惠赋能"],
      overview: "平台聚焦中小企业在政策获取、能力提升、资源对接和经营发展中的共性需求，汇聚行业公共信息与专业服务资源，形成清晰的服务导航和需求反馈渠道。通过线上信息服务与线下专业活动协同，提升企业获取适用资源的便利度。",
      audience: "机械行业中小微企业、创新型企业及相关服务机构",
      value: "降低企业寻找政策与服务资源的成本，促进需求与专业能力有效对接",
      capabilities: [
        ["政策信息服务", "分类汇聚涉企政策、申报通知和公益解读，帮助企业快速定位适用信息。"],
        ["公共资源汇聚", "集中展示培训、技术、人才、金融和市场等普惠型服务资源。"],
        ["企业需求对接", "建立需求提交、分类流转和服务反馈机制，促进供需双方有效沟通。"],
        ["成长赋能支持", "围绕数字化、质量、合规和市场拓展等主题组织诊断、培训与交流。"]
      ],
      process: ["企业注册", "需求识别", "资源匹配", "跟踪反馈"]
    },
    computing: {
      label: "专项平台",
      title: "行业智算服务平台",
      lead: "面向行业智能计算需求，提供计算资源、任务支撑、应用接入和运行监测等基础服务。",
      image: "./assets/unsplash-cloud-server.jpg",
      imageAlt: "云端算力与服务器资源",
      tags: ["算力资源", "任务服务", "应用接入", "运行监测"],
      overview: "平台围绕机械行业人工智能与高性能计算应用的基础需求，对计算资源、任务运行和应用服务进行统一管理。通过标准化接入和运行监测能力，为研发分析、模型训练和智能应用验证等场景提供稳定、可管理的基础环境。",
      audience: "开展智能研发、数据分析和人工智能应用的企业、院校与科研机构",
      value: "提高计算资源使用效率，降低智能应用基础环境的建设和管理复杂度",
      capabilities: [
        ["计算资源管理", "统一纳管计算、存储和网络资源，支持资源查看、分配和使用状态管理。"],
        ["任务服务支撑", "为分析计算、模型训练和批量任务提供提交、调度与结果管理能力。"],
        ["应用接入管理", "通过规范接口支持行业应用和工具接入，形成可扩展的服务入口。"],
        ["运行状态监测", "持续展示资源负载、任务状态和异常信息，辅助平台稳定运行。"]
      ],
      process: ["需求评估", "资源配置", "应用接入", "运行保障"]
    }
  };

  if (page === "platform-detail") {
    const profile = platformProfiles[params.get("id")] || platformProfiles.saas;
    const profileTitle = document.getElementById("platform-profile-title");
    const profileLead = document.getElementById("platform-profile-lead");
    const profileLabel = document.getElementById("platform-profile-label");
    const profileImage = document.getElementById("platform-profile-image");
    const profileTags = document.getElementById("platform-profile-tags");
    const profileOverview = document.getElementById("platform-profile-overview");
    const profileAudience = document.getElementById("platform-profile-audience");
    const profileValue = document.getElementById("platform-profile-value");
    const profileCapabilities = document.getElementById("platform-profile-capabilities");
    const profileProcess = document.getElementById("platform-profile-process");

    document.title = profile.title + " - 机械工业信息中心";
    profileTitle.textContent = profile.title;
    profileLead.textContent = profile.lead;
    profileLabel.textContent = profile.label;
    profileImage.src = profile.image;
    profileImage.alt = profile.imageAlt;
    profileTags.innerHTML = profile.tags.map(function (tag) {
      return `<li>${tag}</li>`;
    }).join("");
    profileOverview.textContent = profile.overview;
    profileAudience.textContent = profile.audience;
    profileValue.textContent = profile.value;
    profileCapabilities.innerHTML = profile.capabilities.map(function (capability) {
      return `<article><h3>${capability[0]}</h3><p>${capability[1]}</p></article>`;
    }).join("");
    profileProcess.innerHTML = profile.process.map(function (step) {
      return `<li><span>${step}</span></li>`;
    }).join("");
  }

  const standardServiceWorkflow = ["需求调研", "方案规划", "专业咨询", "标准产品部署实施", "定制化开发", "运行维护", "数据支撑", "优化迭代"];
  const serviceProfiles = {
    software: {
      category: "信息技术服务",
      title: "软件产品及定制开发服务",
      lead: "面向机关单位与行业协会提供标准化软件产品、定制化开发及全流程实施服务。",
      image: "./assets/unsplash-software-development.jpg",
      imageAlt: "软件开发与信息系统建设",
      offeringsTitle: "软件产品",
      offerings: [
        {
          title: "协同办公系统",
          image: "./assets/unsplash-engineering-meeting.jpg",
          imageAlt: "团队围桌讨论与协同工作",
          summary: "适配机关单位的一体化数字化办公平台，集系统管控、综合办公、财务管控、多系统集成于一体。",
          intro: [
            "协同办公系统是适配机关单位的一体化数字化办公平台，集系统管控、综合办公、财务管控、多系统集成于一体，全方位赋能规范化、智能化办公。",
            "平台支持用户、角色、流程及会员权限精细化管理，筑牢系统安全运维根基。覆盖公文流转、用印归档、会务用车、差旅审批、调研简报、多渠道信息发布及三审三校等全流程行政办公场景，实现办公业务线上闭环管控。",
            "系统集成合同管理、费用报销、开票收款、劳务薪酬、物资借用等财务配套功能，联动会议数据自动归集，实现业财数据互通、合规可控。同时深度对接用友 U8、门户、会议、项目申报、大数据服务等各类系统，一站式整合业务入口，破除信息孤岛，高效提升政企办公协同效能，助力数字化办公体系搭建。"
          ],
          features: [
            ["系统管理", "流程管理、角色管理、APP 管理、用户管理、功能管理、会员功能管理、会员角色授权、会员流程管理。"],
            ["综合办公", "发文管理、收文管理、签报管理、公文归档、用印管理、每周要报、会议室预订、课题管理、调研信息、工作简报、法人信息借用、出差管理、出国（境）申请、会领导工作安排、用车管理、信息发布及三审三校。"],
            ["财务管理", "合同管理、物品借用、会议数据、劳务报酬、财务报销、收款开票。"],
            ["集成对接", "工资查询直接对接用友 U8；集成门户、会员、会议、大数据服务、问卷、项目申报等各类管理后台入口。"]
          ]
        },
        {
          title: "门户网站管理系统",
          image: "./assets/unsplash-software-development.jpg",
          imageAlt: "电脑上的网站与开发工作界面",
          summary: "集权威资讯展示、多业务集成、智能后台管控于一体的综合性官方服务平台。",
          intro: [
            "协会门户网站是集权威资讯展示、多业务集成、智能后台管控于一体的综合性官方服务平台，是协会对外形象展示、信息公开与便民服务的核心窗口。",
            "门户汇聚协会简介、通知公告、新闻动态、政策法规、行业资讯、热门活动等核心内容，支持各分会主页及资讯独立展示，全方位、规范化对外输出行业信息与协会动态。平台设置一站式集成入口，可便捷跳转协同办公、会员、会议、项目申报、大数据服务等业务系统，实现多平台互联互通、高效访问。",
            "后台内嵌于协同办公系统，支持网站内容、基础信息、备案信息的自主维护，搭配管理员、编辑、审核员分级权限管理，规范内容发布审核流程。同时具备网站访问量、访客地域、页面热度、用户行为等数据统计分析能力，以数据赋能协会精细化运营与科学决策。"
          ],
          features: [
            ["门户首页", "协会简介、通知公告、新闻动态、政策法规、行业资讯等信息展示，分会主页及资讯展示，热门活动展示等。"],
            ["集成入口", "一站式跳转协同办公、会员、会议、问卷、项目申报、大数据服务等平台。"],
            ["后台管理", "集成在协同办公系统内，支持协会简介、资讯信息维护；网站名称、版权信息、联系方式、ICP 备案等信息设置；管理员、编辑、审核员分级授权；访问量、访客地域、页面热度、用户行为分析。"]
          ]
        },
        {
          title: "会员管理系统",
          image: "./assets/unsplash-manufacturing-team.jpg",
          imageAlt: "制造企业团队在生产现场工作",
          summary: "面向协会的一体化数字化会员服务与管理平台，覆盖公开展示、会员自助和后台管控。",
          intro: [
            "会员管理系统是面向协会打造的一体化数字化会员服务与管理平台，集公开展示、会员自助、后台管控于一体，助力会员服务便捷化、会籍管理规范化、运营管理数据化。",
            "系统搭建公开会员门户，集中展示协会信息、分支机构介绍、入会流程、会员名录及会员活动，支持线上入会申请，为外部意向人员及会员提供透明、直观的信息公示与服务通道。",
            "内置专属会员中心，为在册会员提供资讯查看、会籍管理、线上缴费、项目申报、资料报送等一站式自助服务，简化办事流程，提升会员服务体验。",
            "管理后台支持全流程会籍维护、会费管控、会员信息查询与资讯运维，搭配可视化会员数据大屏，可实时统计、分析会员数据，实现会员资源精细化、数字化管理，为协会运营决策提供数据支撑。"
          ],
          features: [
            ["会员门户", "协会简介及联系方式、入会流程图、各分支机构简介及联系方式、会员名录、会员活动、入会申请等。"],
            ["会员中心", "会员资讯、我的会籍、会费缴纳、会员申报、会员报送等。"],
            ["会员管理后台", "会籍管理、会费管理、会籍查询、会员大屏、会员资讯、系统设置。"]
          ]
        },
        {
          title: "会议管理系统",
          image: "./assets/unsplash-conference.jpg",
          imageAlt: "行业会议现场",
          summary: "贯通会前筹备、会中管控、会后复盘全业务链条的一站式数字化会议运营平台。",
          intro: [
            "会议管理系统是一站式全流程数字化会议运营平台，贯通会前筹备、会中管控、会后复盘全业务链条，适配协会各类会议标准化、智能化管理需求。",
            "平台支持自助建会、日程配置、报名缴费、平行会议、角色权限等会前灵活配置，高效完成会议前期筹备。会中整合嘉宾管理、订单收费、发票管控、多场景签到、实时数据监控能力，实现现场与财务一体化管控。",
            "系统支持会后问卷调研、云相册、资料下载，并自动汇总签到、缴费、收益数据，智能生成会议报告，实现数据化复盘。依托会议门户与报名小程序，提供会议展示、在线报名、订单管理、二维码参会核验等自助服务，全面简化参会流程，提升会议运营效率与服务体验。"
          ],
          features: [
            ["会前", "自助建会、日程、报名项定义、报名费用定义、平行会议管理、人员角色管理等会前基本信息设置。"],
            ["会中", "参会嘉宾、身份费用标记、会员单位管理、订单管理、收费管理、发票管理、签到场景设置、会议签到、就餐签到、即时数据监控等会中信息管理。"],
            ["会后", "调查问卷、云相册、资料下载、注册签到情况、缴费收益情况、会议整体情况报告等会后数据统计查询。"],
            ["会议门户及报名小程序", "当前会议及历史会议信息展示、参会报名、我的订单、我的参会二维码等。"]
          ]
        },
        {
          title: "大数据服务管理系统",
          image: "./assets/about-work-data.png",
          imageAlt: "数据分析与可视化界面",
          summary: "面向协会的全域数据采集、融合、监控与安全管理一体化平台。",
          intro: [
            "大数据服务管理系统是面向协会的全域数据采集、融合、监控与安全管理一体化平台。系统支持自定义表单、智能校验、任务管控与进度追踪，实现数据规范高效采集；可按多维度聚合统计各类业务数据，打通内部各系统数据，并支持对接第三方及政务平台，实现数据双向互通。",
            "依托可视化数据大屏，全景展示会员、会议、项目、审批等核心运营指标，支持实时刷新与异常预警，满足多场景数据可视化监控需求。同时配备数据备份、权限管控、日志留存等安全能力，全面保障数据合规安全，以数据赋能协会精细化运营与智能化决策。"
          ],
          features: [
            ["数据填报", "填报表单自定义、数据校验，填报任务下发、催报、撤回、重填、锁定，填报进度实时查看。"],
            ["数据汇总", "按时间、地区、行业、规模、分支机构、会员类型等维度汇总填报数据，对接协同办公、会员、会议、问卷、项目申报等系统数据，支持与第三方平台或政府系统接口对接，数据双向同步。"],
            ["数据大屏", "全局可视化展示会员总量、会议举办、项目申报、审批流转等核心指标；监控数据实时刷新，异常数据预警；协会总览、分支机构、会员分析、运营监控等多场景大屏展示。"],
            ["数据管理", "数据备份、权限管理、日志留存等。"]
          ]
        },
        {
          title: "问卷管理系统",
          image: "./assets/unsplash-data-analysis.jpg",
          imageAlt: "屏幕上的调查数据分析图表",
          summary: "集可视化表单搭建、智能考试测评、灵活发布推送、数据统计分析于一体。",
          intro: [
            "问卷管理系统是集可视化表单搭建、智能考试测评、灵活发布推送、数据统计分析于一体的智能化调研测评平台。",
            "系统支持问卷界面自定义配色、水印、版式样式，搭载单选、多选、矩阵、评分、手机号核验等丰富基础及高级组件，可配置审批、会签、或签等流程规则，满足各类复杂表单采集场景。",
            "平台支持题库导入、随机抽题、题目乱序、分值权重自定义及答案解析配置，可实现标准化考试测评与多维度精准评分，自动生成详细测评报告。支持问卷题库管理、证书生成打印、分享传播、提交跳转等个性化发布配置，适配多样化调研与测评推广需求。",
            "配套完善的系统管理与数据统计能力，支持组织架构、短信邮件、支付存储等参数配置，可对问卷回收量、地域分布、设备占比等数据进行智能分析，助力协会高效开展调研评估、数据收集与成果分析工作。"
          ],
          features: [
            ["问卷设置", "头图、背景、按钮、字体、标题颜色、答案颜色、水印等问卷界面设置；单行文本、多行文本、单选、多选、下拉、日期时间等基础组件设置；子表单、多类型矩阵组件、评分、手机号验证等高级组件设置；审批流程、会签、或签等流程设置。"],
            ["考试测评", "自定义分数、答案解析、题干说明、题目随机顺序、题库内随机抽题、交卷时间等试卷设置；查看测评成绩、自定义测评维度、设置各项评分权重、展示详细测评结果报告等。"],
            ["问卷或试卷发布", "题库管理、题库导入、证书生成、证书打印、提交后跳转，以及问卷分享图标、标题、描述等设置。"],
            ["系统管理", "组织架构设置，短信、邮件、存储、支付、地图等设置，协作管理，以及回收量、地域、设备、占比分析等数据统计。"]
          ]
        },
        {
          title: "项目申报系统",
          image: "./assets/unsplash-industry-research.jpg",
          imageAlt: "项目资料与规划图纸讨论",
          summary: "实现表单自定义、在线申报、流程审核、专家评审、结果公示与成果展示全流程闭环管理。",
          intro: [
            "项目申报系统是一体化、全线上化的项目申报与评审管控平台，实现表单自定义、在线申报、流程审核、专家评审、结果公示与成果展示全流程闭环管理。系统支持拖拽式表单可视化配置，提供文本、附件上传等多类型字段，可灵活设置必填规则、字数与文件大小限制，快速适配各类项目申报填报规范。",
            "平台支持用户在线填报、草稿保存、一键提交材料，申报进度、审核意见实时同步，驳回修改全程可追溯；支持线上专家评审，全过程留痕存档，保障评审工作公正透明。管理人员可自主发布申报项目、动态监控申报数据、灵活调整表单内容，并通过多渠道推送评审结果，同时支持优秀项目成果公开展示，有效规范项目申报管理流程、提升申报评审工作效率与标准化水平。"
          ],
          features: [
            ["项目申报", "支持在线填报、草稿保存、数据同步、一键材料提交、在线审核、驳回标注、线上专家评审及评审结果在线公示。"],
            ["申报表单配置", "提供文本、文件上传等多类型字段；拖拽式定制表格、单列等多种表单样式；配置必填项、字数或文件大小限制等填报规则。"],
            ["申报管理", "可配置参数并发布申报项目；实时监控申报数据、支持表单调整；评审结果通过多渠道推送；支持优秀项目成果公开展示。"]
          ]
        },
        {
          title: "专家库管理系统",
          image: "./assets/unsplash-engineering-meeting.jpg",
          imageAlt: "专业人员围桌交流",
          summary: "实现专家信息入库审核、动态维护、精准检索与线上沟通全流程管控。",
          intro: [
            "专家库管理系统是专业化、规范化的专家资源归集与信息运维管理平台，实现专家信息入库审核、动态维护、精准检索与线上沟通全流程管控。系统支持企业自主新增、编辑、临时保存专家信息，资料提交后通过平台在线审核即可纳入专家库，支持专家资料动态更新、变更重审及出库移除，保障专家资源信息真实有效、实时同步。",
            "平台提供多维度筛选与关键词检索能力，可按专业领域、层级快速匹配所需专家，支持查看专家单位、研究成果等详细资料。系统内置线上消息转达功能，可发起专家沟通并留存消息记录，实现专家资源精细化管理、高效检索与合规对接，助力评审资源统筹调配、提升项目评审专业化水平。"
          ],
          features: [
            ["专家信息管理", "企业可添加、临时保存并修改专家信息；提交信息后经平台在线审核，审核通过后纳入专家库；企业可维护更新专家信息，变更需重审，也可移除专家。"],
            ["专家信息检索", "支持按专业领域、层级筛选，或关键词检索专家；可查看专家姓名、单位、研究成果等详情；支持在线发送联系消息，企业代为转达并可查看消息记录。"]
          ]
        },
        {
          title: "知识库管理系统",
          image: "./assets/unsplash-data-center.jpg",
          imageAlt: "承载数字资料的数据中心服务器",
          summary: "集文档存储、知识沉淀、档案归档、协同共享于一体的数字化知识管理平台。",
          intro: [
            "知识库管理系统是集文档存储、知识沉淀、档案归档、协同共享于一体的数字化知识管理平台。",
            "系统支持多格式文档批量上传、断点续传、在线预览与协同编辑，可自定义分类权限，依托全文检索快速调取资料，实现文档精细化管控。",
            "平台支持知识分类审核、点赞评论、分享传播及历史版本留存，配备文件过期提醒功能，助力内部知识高效沉淀与流转复用。同时支持各类重要文件归档，关联业务流程附件，提供线上借阅查询服务，无用档案可审批销毁且全程留痕，全面规范单位知识档案管理体系，实现资源集约化、标准化、安全化管理。"
          ],
          features: [
            ["文档管理", "支持多格式文档批量上传、断点续传及权限内下载；可自定义分类与权限，支持在线预览、协同编辑，全文检索快速定位文档。"],
            ["知识库管理", "按知识类型分类管理与审核，支持点赞、评论、分享，保留历史版本，提供过期提醒。"],
            ["档案管理", "支持各类重要文件归档，关联流程附件，支持员工借阅与查询；过期无用档案经审批后销毁，过程留痕。"]
          ]
        },
        {
          title: "TALKIN",
          image: "./assets/about-work-network.png",
          imageAlt: "城市通信网络连接",
          summary: "专为机构打造的安全高效即时通讯协同平台，集即时沟通、后台管控、全域安全防护于一体。",
          intro: [
            "TALKIN 是专为机构打造的安全高效即时通讯协同平台，集成即时沟通、后台管控、全域安全防护能力，满足单位日常办公沟通、团队协作与数据安全管理需求。平台具备完善的即时通讯能力，支持消息收发、已读未读提醒、消息撤回、截图编辑、群投票、备忘提醒、全局搜索及精细化群管理，配套表情包、专属网盘等实用功能，丰富办公协作场景、提升沟通效率。",
            "系统搭载标准化后台管理体系，支持人员信息、组织架构、企业存储统一管控，全程留存操作日志，实现办公人员与资源规范化运维。同时配备全链路多重加密、应用锁、分级权限配置、设备管理、风险上报等多重安全机制，全方位保障企业通讯数据可控、可溯源、高安全，构建合规、稳定、安全的一体化办公通讯协作体系。"
          ],
          features: [
            ["即时通讯", "丰富的消息发送、消息已读未读提醒、聊天撤回、截图、群投票、表情包、消息提醒备忘、在线标识、全局搜索、群管理、网盘。"],
            ["后台管理", "人员管理、组织架构管理、企业存储管理、操作日志。"],
            ["安全机制", "全链路多重加密、应用锁机制、灵活配置权限、设备管理、企业数据可控、风险上报。"]
          ]
        }
      ],
      technologyLead: "统一技术底座 × 统一数据平台 × 统一用户体系——重新定义企业级门户，让数字化从“展示窗口”迈向“增长引擎”。",
      technology: [
        ["统一技术底座", "以微服务 + 容器化为核心，构建高可用、弹性伸缩的云原生架构。就像为业务安上了“自动变速箱”——高并发场景下自动扩容，故障时秒级自愈，让您的业务增长再无后顾之忧。"],
        ["统一数据平台", "打通 CRM、ERP 等多源数据孤岛，构建企业级“数据大脑”。基于实时画像与智能推荐引擎，实现“千人千面”的精准服务，让每一次点击都产生价值。"],
        ["统一用户体系", "基于 OAuth 2.0 标准实现单点登录（SSO），一次认证，全网通行。告别多系统反复登录的繁琐，让员工、伙伴与客户的每一次访问都安全、高效、无缝。"]
      ],
      technologyBenefits: [
        ["高可用", "系统吞吐量提升 40%，峰值流量丝滑应对。"],
        ["智能化", "数据驱动决策，推荐准确率提升 35%。"],
        ["降本增效", "站群统一管理，运维成本降低 50% 以上。"],
        ["极致体验", "多端一致，跨屏无缝，客户满意度显著提升。"]
      ],
      workflow: [
        { title: "调研与咨询", items: [["需求调研", "深入了解客户的业务现状、痛点与信息化目标，通过访谈、问卷、现场走访等方式，全面梳理业务流程与功能需求。"], ["现状评估", "对客户现有信息系统、数据资产、技术架构进行评估，识别优化空间与潜在风险。"], ["咨询建议", "基于调研结果，提供专业的信息化规划建议与建设路径参考，帮助客户明确建设优先级与投资回报预期。"]] },
        { title: "规划与设计", items: [["方案设计", "结合客户实际需求与标准产品能力，制定详细的技术方案与实施路线图，明确功能范围、时间节点与里程碑。"], ["架构设计", "完成系统整体架构设计，包括技术选型、数据模型设计、接口规范定义，确保系统的可扩展性与安全性。"], ["方案评审", "与客户共同评审设计方案，确认需求边界与交付标准，形成书面确认文档。"]] },
        { title: "标准产品部署实施", items: [["环境搭建", "完成服务器、网络、数据库等基础环境的部署与配置，确保运行环境稳定可靠。"], ["产品部署", "基于标准产品进行安装部署，完成基础配置与初始化，快速搭建可运行的系统框架。"], ["数据迁移", "协助客户完成历史数据的清洗、转换与迁移，确保业务数据的完整性和连续性。"], ["系统联调", "完成各子系统之间的集成联调，确保门户、OA、会员、会议等模块间的数据互通与流程协同。"]] },
        { title: "定制化开发", items: [["需求细化", "在标准产品基础上，与客户进一步细化个性化需求，明确定制功能的业务逻辑与交互方式。"], ["功能开发", "按照确认的需求规格进行定制化开发，包括特殊业务流程实现、第三方系统对接、自定义报表与大屏开发等。"], ["集成开发", "完成与外部系统的集成对接，实现数据双向同步与统一入口访问。"], ["代码评审", "对定制开发代码进行质量评审，确保代码规范、性能优化与安全合规。"]] },
        { title: "测试与验收", items: [["功能测试", "对标准产品及定制开发功能进行全面测试，覆盖功能完整性、边界条件与异常场景。"], ["性能测试", "进行压力测试与性能调优，确保系统在高并发场景下的稳定运行。"], ["安全测试", "开展漏洞扫描与安全渗透测试，修复安全隐患，保障系统安全。"], ["用户验收", "配合客户进行用户验收测试，根据反馈进行优化调整，直至满足验收标准。"]] },
        { title: "上线与运维", items: [["上线部署", "制定详细的上线方案与回滚预案，确保系统平滑过渡至生产环境，最大限度减少业务中断。"], ["培训赋能", "为客户提供系统操作培训与管理员培训，配套提供操作手册与视频教程，帮助客户团队快速上手。"], ["运维保障", "提供 7×24 小时全天候运维支持，涵盖应用系统保障、数据资产管理、安全巡检与性能调优，确保业务连续稳定运行。"]] },
        { title: "数据支撑", items: [["数据分析", "基于系统积累的业务数据，提供多维度的数据分析与可视化展示，为管理决策提供数据依据。"], ["数据大屏", "通过数据大屏实时展示核心业务指标，实现运营状态的可视化监控与异常预警。"], ["数据报告", "定期输出数据分析报告，揭示业务趋势与潜在问题，辅助客户优化运营策略。"]] },
        { title: "优化迭代", items: [["持续优化", "根据系统运行反馈与业务发展需求，持续对系统进行功能优化与性能调优。"], ["版本迭代", "跟踪技术发展趋势，定期推出功能升级与版本迭代，确保系统始终保持技术领先。"], ["需求演进", "建立长效的需求收集与反馈机制，敏捷响应客户新增需求，推动系统持续进化。"]] }
      ],
      aiService: {
        title: "新技术（AI）服务",
        lead: "下一代人工智能技术服务，为企业提供可落地、可量化、可持续的 AI 能力",
        slogan: "AI 新技术服务・智能重构效率，技术创造价值",
        description: [
          "基于大模型、多模态、智能体与自动化流程，为企业提供一站式 AI 解决方案",
          "从咨询、部署到运营，全程陪伴，快速见效"
        ],
        coreServices: [
          {
            title: "大模型定制与私有化部署",
            items: ["企业专属垂直大模型训练与微调", "本地 / 私有云部署，数据安全可控", "知识库接入、意图理解、问答体系构建"]
          },
          {
            title: "AIGC 全栈内容生产",
            items: ["文本生成：文案、简报、报告、话术", "图像 / 视频生成：营销素材、产品可视化", "多模态内容一键产出，降本增效"]
          },
          {
            title: "AI 智能体与流程自动化",
            items: ["智能客服、智能助理、业务执行体", "RPA+AI 自动化，替代重复人工操作", "跨系统协同，流程闭环，7×24 小时运行"]
          },
          {
            title: "数据智能与决策分析",
            items: ["数据清洗、标签体系、智能洞察", "预测分析、异常检测、趋势研判", "可视化报表，辅助经营与管理决策"]
          },
          {
            title: "AI 数字人与多模态交互",
            items: ["虚拟主播、数字员工、品牌代言人", "语音交互、实时播报、视频生成", "全渠道适配：官网、APP、小程序、大屏"]
          }
        ],
        advantages: [
          { title: "前沿技术栈", description: "大模型、多模态、知识图谱、强化学习、计算机视觉、NLP" },
          { title: "安全合规", description: "数据不出域、隐私计算、等保合规、企业级加密" },
          { title: "快速落地", description: "标准化产品 + 定制化开发，最短 7 天上线可用" },
          { title: "全生命周期服务", description: "需求评估→方案设计→部署上线→迭代优化→运维保障" },
          { title: "行业深度适配", description: "零售、金融、制造、文旅、政企、教育、医疗等场景化方案" }
        ],
        scenarios: [
          { title: "企业营销", description: "AI 内容创作、智能投放、用户洞察" },
          { title: "客户服务", description: "智能客服、智能外呼、满意度提升" },
          { title: "生产运营", description: "流程自动化、质检、设备预测维护" },
          { title: "办公提效", description: "文档处理、会议纪要、审批助手" },
          { title: "品牌传播", description: "数字人直播、短视频矩阵、IP 打造" }
        ],
        modes: [
          {
            title: "标准版",
            items: ["标准化 AI 产品接入", "基础 API 调用", "在线技术支持"],
            suitable: "适合：中小企业快速上线"
          },
          {
            title: "企业版",
            items: ["私有化部署", "专属模型微调", "定制化功能开发", "专属客户经理 + 运维"],
            suitable: "适合：中大型企业 / 数据敏感行业"
          },
          {
            title: "定制方案",
            items: ["全栈 AI 解决方案", "联合研发与共建", "年度服务与战略合作"],
            suitable: "适合：集团、政府、特殊行业"
          }
        ]
      },
      ai: "新技术（AI）服务相关内容待补充。",
      aiNote: "文档仅提供章节标题，具体服务内容与可公开范围待确认。",
      casesLead: "以下为内容模板列示的客户单位，具体项目名称、成果内容及公开范围待确认。",
      cases: [
        "中国机械工业联合会", "机械工业教育发展中心", "中国环保机械行业协会", "中国电工技术学会",
        "中国机械通用零部件工业协会", "中国液压气动密封件工业协会", "中国电器工业协会", "中国重型机械工业协会",
        "中国内燃机工业协会", "中国汽车工业协会", "机械工业价格研究中心", "机械工业人才开发服务中心",
        "机械工业发电设备中心", "机械工业北京电工技术经济研究所", "华信技术检验有限公司", "中国工艺美术协会",
        "北京京城机电控股有限责任公司", "北京理工大学附属中学", "北京机电行业协会", "河南德诺会展服务有限公司"
      ]
    },
    integration: {
      category: "信息技术服务", title: "系统集成服务", lead: "提供信息化硬件、网络系统及软硬件一体化集成部署与联调实施。", image: "./assets/unsplash-data-center.jpg", imageAlt: "数据中心服务器与网络设备",
      offerings: [["基础设施集成", "开展机房、网络和终端设备等基础环境建设。", "具体设备选型、数量与实施范围按项目方案确认。"], ["系统接口集成", "支持多系统之间的接口开发、数据交换和联调实施。", "接口标准、数据边界与安全要求需在调研阶段确认。"], ["部署与交付", "提供安装部署、联合测试、上线支持和基础培训。", "交付方式与验收要求依据项目合同和实施方案执行。"]],
      technology: [["集成架构", "根据现有系统与目标环境设计网络、设备和系统集成架构。"], ["接口规范", "通过明确接口、数据和权限规范降低系统协同复杂度。"], ["运行保障", "结合监测、备份和应急机制保障集成环境稳定运行。"]], workflow: standardServiceWorkflow,
      ai: "可结合运维监测和设备运行数据，探索异常辅助识别、知识检索和故障处置建议等应用。"
    },
    cloud: {
      category: "信息技术服务", title: "云资源服务", lead: "提供云主机、云存储、云网络等资源及云环境部署、迁移和统一管理服务。", image: "./assets/unsplash-cloud-server.jpg", imageAlt: "云端算力与服务器资源",
      offerings: [["云计算资源", "按需求提供云主机、存储和网络等基础资源。", "资源规格、服务周期与可用范围需结合实际方案确认。"], ["云环境部署", "提供应用环境规划、部署配置和迁移适配支持。", "迁移对象、停机窗口和数据安全要求需专项确认。"], ["云资源管理", "开展资源监测、配置调整、备份和安全管理。", "服务等级和响应机制依据具体服务约定执行。"]],
      technology: [["资源管理", "统一管理计算、存储和网络资源的配置与使用状态。"], ["环境隔离", "根据业务和安全需求规划网络、权限与运行环境。"], ["监测备份", "通过运行监测和数据备份提高云环境可靠性。"]], workflow: standardServiceWorkflow,
      ai: "可结合资源监测数据探索容量预测、异常识别和运维知识辅助，具体能力需依据数据条件验证。"
    },
    operations: {
      category: "信息技术服务", title: "信息系统运维服务", lead: "为已上线系统与硬件设备提供日常维护、故障处置、巡检和持续优化支持。", image: "./assets/unsplash-monitoring.jpg", imageAlt: "信息系统运行监测界面",
      offerings: [["日常运行维护", "开展系统巡检、状态检查和问题记录。", "巡检范围与频次根据系统重要程度和服务约定确定。"], ["故障与应急保障", "提供故障排查、恢复支持和应急响应。", "响应级别与处置时限按具体运维方案执行。"], ["性能与版本优化", "支持性能分析、版本迭代、备份和运行优化。", "优化内容需基于运行情况和变更评估实施。"]],
      technology: [["运行监测", "持续关注系统、接口和基础资源的运行状态。"], ["知识沉淀", "记录常见问题、处置过程和运维规范，提高服务连续性。"], ["变更管理", "规范版本、配置和数据变更，降低运行风险。"]], workflow: standardServiceWorkflow,
      ai: "可探索告警归并、知识问答和故障原因辅助分析，不替代必要的人工研判与操作确认。"
    },
    security: {
      category: "信息技术服务", title: "安全合规服务", lead: "围绕网络与数据安全提供防护建设、检测评估、整改和应急支持。", image: "./assets/unsplash-cloud-server.jpg", imageAlt: "安全运行基础设施",
      offerings: [["安全防护建设", "结合系统环境建设必要的安全技术防护措施。", "防护范围与产品选型需根据风险评估确定。"], ["检测与整改", "开展等级保护辅助、漏洞扫描、安全测试和整改支持。", "具体检测方式需在授权范围内实施。"], ["应急与合规支持", "协助建立应急响应和安全管理机制。", "合规结论以具备相应资质的正式评估为准。"]],
      technology: [["身份与权限", "通过身份认证和最小权限原则控制系统访问。"], ["安全监测", "关注网络、应用和数据使用中的异常风险。"], ["备份与响应", "建立必要的备份恢复和事件处置机制。"]], workflow: standardServiceWorkflow,
      ai: "可探索安全事件信息归纳、规则辅助分析和知识检索，输出结果需由安全人员复核。"
    },
    "data-elements": {
      category: "数智化创新赋能服务", title: "数据要素服务", lead: "开展行业数据归集、治理、标准化、产品开发与数据服务配套工作。", image: "./assets/industrial-data.png", imageAlt: "团队开展工业数据分析",
      offerings: [["数据资源归集", "梳理并接入业务与行业数据资源。", "数据来源、权属与使用范围需事先确认。"], ["数据治理", "开展标准、质量、目录和标签等治理工作。", "治理规则需结合业务定义共同制定。"], ["数据产品与服务", "探索数据资源的产品化与服务化应用。", "具体产品及市场化安排待项目确认。"]],
      technology: [["数据底座", "支持多源数据接入、存储与统一管理。"], ["治理体系", "建立数据标准、质量规则和资源目录。"], ["服务接口", "通过受控接口和数据产品支持合规应用。"]], workflow: standardServiceWorkflow,
      ai: "可结合合规数据开展智能分类、质量识别和分析辅助，数据授权与安全边界需优先确认。"
    },
    "digital-evaluation": {
      category: "数智化创新赋能服务", title: "数智化评价咨询服务", lead: "开展转型水平、项目成效、数据治理与安全能力诊断，提供路径规划建议。", image: "./assets/unsplash-data-analysis.jpg", imageAlt: "工业数据指标分析界面",
      offerings: [["转型水平评估", "依据明确指标了解企业数智化建设现状。", "评价指标、样本和结论适用范围需项目确认。"], ["项目成效评价", "分析数智化项目目标、实施过程与应用成效。", "评价依据以项目资料和可验证证据为准。"], ["路径规划咨询", "结合诊断结果提出分阶段优化建议。", "建议不替代企业内部投资与管理决策。"]],
      technology: [["指标体系", "围绕战略、业务、技术、数据和组织等维度设计评价指标。"], ["证据管理", "规范收集访谈、材料与系统数据，支撑评价判断。"], ["分析模型", "通过定量与定性方法形成结构化诊断结果。"]], workflow: standardServiceWorkflow,
      ai: "可用于材料辅助整理、指标信息提取和知识检索，评价结论仍需专业人员综合研判。"
    },
    "digital-training": {
      category: "数智化创新赋能服务", title: "数智化能力培育服务", lead: "提供专题培训、体系化课程与案例研学，支持数智化人才能力建设。", image: "./assets/unsplash-engineering-meeting.jpg", imageAlt: "专业团队开展研究讨论",
      offerings: [["专题培训", "围绕数智化主题开展针对性课程。", "课程主题和对象根据培训需求确定。"], ["体系化能力建设", "结合岗位与业务设计分层课程内容。", "课程体系需结合组织能力现状规划。"], ["案例研学", "通过案例交流和现场研学促进经验理解。", "研学安排及案例可公开范围待确认。"]],
      technology: [["课程体系", "围绕业务场景、工具方法和实践能力组织内容。"], ["学习管理", "支持课程、学员、资料和反馈的过程管理。"], ["效果反馈", "通过学习反馈和应用情况持续优化课程。"]], workflow: standardServiceWorkflow,
      ai: "可探索课程资料检索、学习辅助和内容整理，培训材料需经过专业审核。"
    },
    strategy: {
      category: "行业智库咨询服务", title: "战略规划咨询服务", lead: "面向行业、产业集群、园区和企业提供中长期战略、布局与转型规划支持。", image: "./assets/unsplash-industry-research.jpg", imageAlt: "行业研究与战略分析",
      offerings: [["发展战略规划", "研究发展环境、目标方向与重点任务。", "规划内容需建立在真实资料与充分调研基础上。"], ["产业布局规划", "分析产业基础、空间布局和协同关系。", "相关结论以项目约定范围为准。"], ["数智化顶层设计", "规划数智化建设架构、路径和阶段任务。", "实施优先级需结合资源和业务价值共同确定。"]],
      technology: [["研究框架", "建立宏观、行业、区域与主体相结合的分析框架。"], ["数据支撑", "使用可追溯的数据和资料支持规划判断。"], ["任务体系", "将目标转化为重点工程、实施阶段和保障机制。"]], workflow: standardServiceWorkflow,
      ai: "可用于公开资料辅助检索和文本整理，不直接替代专家研究、访谈与规划论证。"
    },
    "industry-research": {
      category: "行业智库咨询服务", title: "产业研究与决策咨询服务", lead: "开展行业运行、产业趋势、政策法规和重大问题研究，形成专业咨询成果。", image: "./assets/unsplash-engineering-meeting.jpg", imageAlt: "专业团队开展行业研究",
      offerings: [["行业运行分析", "跟踪指标与重点领域变化，分析行业运行态势。", "数据口径和时间范围需在成果中明确。"], ["产业趋势研究", "围绕技术、市场与产业结构开展专题研判。", "趋势判断基于可获得资料和研究假设。"], ["决策咨询成果", "形成咨询报告、建议、白皮书或年度报告。", "成果形式与公开范围按项目约定。"]],
      technology: [["行业数据库", "汇聚研究所需的基础指标与专题资料。"], ["研究方法", "综合使用统计分析、调研访谈与案例研究。"], ["成果管理", "规范资料来源、分析过程和成果版本。"]], workflow: standardServiceWorkflow,
      ai: "可用于资料检索、分类和初步摘要，核心分析与政策判断由研究人员负责。"
    },
    "project-evaluation": {
      category: "行业智库咨询服务", title: "项目评估与论证咨询服务", lead: "提供可行性研究、立项论证、评审验收、绩效评价和优化建议。", image: "./assets/unsplash-precision-lab.jpg", imageAlt: "精密检测与项目评估场景",
      offerings: [["可行性研究", "分析项目必要性、建设条件和实施方案。", "结论依据委托范围和可验证资料形成。"], ["评审与验收支持", "围绕目标、交付和应用情况提供专业论证。", "正式评审权限与责任以相关制度为准。"], ["绩效评价", "分析投入、过程、产出和效果并提出建议。", "指标和证据要求需在评价方案中明确。"]],
      technology: [["评价指标", "建立与项目目标相匹配的评价维度和指标。"], ["证据链", "规范收集立项、实施、交付与应用资料。"], ["分析工具", "通过数据分析和专业判断形成评价结论。"]], workflow: standardServiceWorkflow,
      ai: "可辅助材料校核、信息提取和问题线索整理，评估结论须由专业人员审核。"
    },
    "public-information": {
      category: "行业公共服务", title: "公共信息服务", lead: "发布行业运行、政策标准与基础信息，为行业用户提供普惠信息支持。", image: "./assets/unsplash-newsroom-2.jpg", imageAlt: "行业信息采集与编辑工作",
      offerings: [["行业信息发布", "发布运行监测、发展概况和相关公共信息。", "持续提供行业公共信息服务。"], ["公共资源开放", "提供政策、标准、基础数据和主体名录等信息入口。", "开放范围以数据授权和公开要求为准。"], ["公益解读与科普", "围绕行业重点内容提供中性、专业的解读。", "内容需经审核后发布。"]],
      technology: [["内容管理", "支持信息采编、审核、发布和版本管理。"], ["资源目录", "对公共信息资源进行分类和检索组织。"], ["多端发布", "面向网页和移动端提供一致的信息访问体验。"]], workflow: standardServiceWorkflow,
      ai: "可用于公开信息分类、检索和辅助摘要，发布内容必须经过人工审核。"
    },
    "industry-exchange": {
      category: "行业公共服务", title: "行业交流服务", lead: "策划组织会议、论坛、研讨、成果发布、展会和供需对接等行业活动。", image: "./assets/unsplash-conference.jpg", imageAlt: "行业会议与交流活动",
      offerings: [["会议与论坛", "提供活动策划、报名组织和现场支持。", "活动主题、规模和服务范围按项目确认。"], ["成果发布与展示", "支持成果发布、专题展示和传播组织。", "公开内容与传播范围需经相关方确认。"], ["供需对接", "围绕行业需求组织资源交流与对接活动。", "对接结果取决于参与主体实际意愿。"]],
      technology: [["活动管理", "支持报名、审核、签到、通知和资料管理。"], ["线上服务", "根据需要提供直播、线上会议或专题展示。"], ["资源沉淀", "对活动资料和参与信息进行规范管理。"]], workflow: standardServiceWorkflow,
      ai: "可用于活动资料检索、内容整理和问答辅助，涉及公开发布的信息需人工确认。"
    },
    "inclusive-empowerment": {
      category: "行业公共服务", title: "普惠赋能服务", lead: "面向中小微企业提供公益诊断、政策辅导、共性技术推广和轻量化帮扶。", image: "./assets/unsplash-manufacturing-team.jpg", imageAlt: "中小制造企业生产团队",
      offerings: [["公益诊断", "围绕企业共性问题提供基础分析与建议。", "诊断范围和成果形式需结合服务安排确定。"], ["政策与事务辅导", "提供政策申报、资质和基层合规相关信息支持。", "最终申报和认定结果以主管部门为准。"], ["共性能力推广", "开展技术、管理和数字化等普惠型推广活动。", "具体主题与服务对象按年度安排确定。"]],
      technology: [["企业服务档案", "在授权范围内记录企业需求与服务过程。"], ["资源匹配", "根据需求分类匹配政策、活动和专业资源。"], ["服务反馈", "收集服务结果与建议，持续改进普惠服务。"]], workflow: standardServiceWorkflow,
      ai: "可用于政策信息检索和基础问答辅助，具体申报与合规判断仍需依据正式文件。"
    }
  };

  const generatedServiceDetails = generatedContent.generated === true
    ? (generatedContent.serviceDetails || {})
    : null;
  const requestedServiceCode = params.get("id") || "";
  const generatedService = requestedServiceCode === "software" ? null : generatedServiceDetails && generatedServiceDetails[requestedServiceCode];

  if (page === "service-detail" && generatedService && generatedService.content) {
    const service = generatedService;
    const title = service ? service.title : "暂无服务详情";
    const category = service ? service.category : "核心业务";
    const lead = service ? service.lead : "该服务尚未发布详情内容。";
    document.title = title + " - 机械工业信息中心";
    document.getElementById("service-profile-category").textContent = category;
    document.getElementById("service-profile-label").textContent = category;
    document.getElementById("service-profile-title").textContent = title;
    document.getElementById("service-profile-lead").textContent = lead;
    const serviceImage = document.getElementById("service-profile-image");
    serviceImage.src = rootUrl(service && service.image ? service.image : "assets/miic-placeholder.png");
    serviceImage.alt = service && service.imageAlt ? service.imageAlt : title;
    document.getElementById("service-offerings-title").textContent = "服务详情";
    document.getElementById("service-product-overview").hidden = true;
    document.getElementById("service-profile-offerings").innerHTML = service && service.content
      ? `<article class="static-article__content">${service.content}</article>`
      : '<p class="static-empty">暂无已发布服务详情</p>';
    ["service-profile-technology-section", "service-profile-workflow-section", "service-profile-ai-section", "service-profile-cases-section"].forEach(function (id) {
      const section = document.getElementById(id);
      if (section) section.hidden = true;
    });
    document.getElementById("service-profile-anchor-links").innerHTML = `
      <a class="is-active" href="#service-offerings-section" aria-current="location">服务详情</a>
      <a href="#service-profile-contact-section">服务咨询</a>
    `;
  }

  if (page === "service-detail" && !(generatedService && generatedService.content)) {
    const service = serviceProfiles[params.get("id")] || serviceProfiles.software;
    const isProductCatalog = service.offerings.some(function (offering) { return !Array.isArray(offering); });
    document.body.classList.toggle("service-profile--catalog", isProductCatalog);
    document.title = service.title + " - 机械工业信息中心";
    document.getElementById("service-profile-category").textContent = service.category;
    document.getElementById("service-profile-label").textContent = service.category;
    document.getElementById("service-profile-title").textContent = service.title;
    document.getElementById("service-profile-lead").textContent = service.lead;
    const serviceImage = document.getElementById("service-profile-image");
    serviceImage.src = service.image;
    serviceImage.alt = service.imageAlt;
    document.getElementById("service-offerings-title").textContent = service.offeringsTitle || "服务内容";
    document.getElementById("service-profile-technology-title").textContent = isProductCatalog ? "技术与服务能力" : "核心能力";

    const productOverview = document.getElementById("service-product-overview");
    const productOverviewList = document.getElementById("service-product-overview-list");
    if (isProductCatalog) {
      productOverview.hidden = false;
      productOverviewList.innerHTML = service.offerings.map(function (offering, index) {
        return `<button type="button" data-product-target="service-product-${index}"><span aria-hidden="true"></span>${offering.title}</button>`;
      }).join("");
    } else {
      productOverview.hidden = true;
      productOverviewList.innerHTML = "";
    }

    document.getElementById("service-profile-offerings").innerHTML = service.offerings.map(function (offering, index) {
      if (!Array.isArray(offering)) {
        const introduction = offering.intro.map(function (paragraph) {
          return `<p>${paragraph}</p>`;
        }).join("");
        const features = offering.features.map(function (feature) {
          return `<li><strong>${feature[0]}</strong><span>${feature[1]}</span></li>`;
        }).join("");
        const keywords = offering.features.map(function (feature) {
          return `<span>${feature[0]}</span>`;
        }).join("");
        return `
          <details class="service-offering service-offering--rich" id="service-product-${index}">
            <summary aria-expanded="false">
              <span class="service-offering__summary"><strong>${offering.title}</strong><small>${offering.summary}</small><span class="service-offering__keywords">${keywords}</span></span>
              <img class="service-offering__image" src="${offering.image}" alt="${offering.imageAlt}" loading="lazy">
              <b aria-hidden="true">+</b>
            </summary>
            <div class="service-offering__body">
              <section class="service-offering__intro" aria-labelledby="service-product-${index}-intro">
                <h3 id="service-product-${index}-intro">产品介绍</h3>
                ${introduction}
              </section>
              <section class="service-offering__features" aria-label="${offering.title}完整功能">
                <h3>主要功能</h3>
                <ul>${features}</ul>
              </section>
            </div>
          </details>
        `;
      }
      return `
        <details class="service-offering">
          <summary aria-expanded="false">
            <span class="service-offering__icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M4 5.5h16v13H4zM8 9h8M8 13h5"/></svg></span>
            <span><strong>${offering[0]}</strong><small>${offering[1]}</small></span>
            <b aria-hidden="true">+</b>
          </summary>
          <div><p>${offering[2]}</p></div>
        </details>
      `;
    }).join("");

    const offeringDetails = Array.from(document.querySelectorAll(".service-offering"));
    offeringDetails.forEach(function (details) {
      const summary = details.querySelector(":scope > summary");
      function syncOfferingState() {
        summary.setAttribute("aria-expanded", String(details.open));
      }
      details.addEventListener("toggle", syncOfferingState);
      syncOfferingState();
    });

    productOverviewList.addEventListener("click", function (event) {
      const trigger = event.target.closest("[data-product-target]");
      if (!trigger) return;
      const product = document.getElementById(trigger.dataset.productTarget);
      if (!product) return;
      product.open = true;
      product.scrollIntoView({ behavior: "smooth", block: "start" });
      window.setTimeout(function () {
        product.querySelector(":scope > summary").focus({ preventScroll: true });
      }, 350);
    });

    document.getElementById("service-profile-technology-lead").textContent = service.technologyLead || "以规范、稳定和可持续演进的技术能力支撑服务实施。";
    document.getElementById("service-profile-technology").innerHTML = service.technology.map(function (item) {
      return `<article><span aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M4 17V7l8-4 8 4v10l-8 4-8-4Zm8-14v18M4 7l8 4 8-4"/></svg></span><div><h3>${item[0]}</h3>${item[1] ? `<p>${item[1]}</p>` : ""}</div></article>`;
    }).join("");
    const technologyBenefits = document.getElementById("service-profile-technology-benefits");
    if (service.technologyBenefits && service.technologyBenefits.length) {
      technologyBenefits.hidden = false;
      technologyBenefits.innerHTML = service.technologyBenefits.map(function (item) {
        return `<li><strong>${item[0]}</strong><span>${item[1]}</span></li>`;
      }).join("");
    } else {
      technologyBenefits.hidden = true;
      technologyBenefits.innerHTML = "";
    }

    const detailedWorkflow = service.workflow.some(function (step) { return typeof step === "object"; });
    document.getElementById("service-profile-workflow-section").classList.toggle("service-profile-workflow--detailed", detailedWorkflow);
    document.getElementById("service-workflow-overview").innerHTML = service.workflow.map(function (step) {
      const title = typeof step === "object" ? step.title : step;
      return `<li><span class="service-workflow-overview__dot" aria-hidden="true"></span><strong>${title}</strong></li>`;
    }).join("");
    const workflowFull = document.getElementById("service-workflow-full");
    workflowFull.hidden = !detailedWorkflow;
    document.getElementById("service-profile-workflow").innerHTML = service.workflow.map(function (step, index) {
      if (typeof step === "object") {
        const items = step.items.map(function (item) {
          return `<div><dt>${item[0]}</dt><dd>${item[1]}</dd></div>`;
        }).join("");
        return `<li class="service-workflow-step"><span class="service-workflow-step__marker" aria-hidden="true"></span><header><h3>${step.title}</h3></header><dl>${items}</dl></li>`;
      }
      return `<li><span>${step}</span></li>`;
    }).join("");

    const aiSection = document.getElementById("service-profile-ai-section");
    const aiContent = document.getElementById("service-profile-ai-content");
    if (service.aiService) {
      const aiCoreIcons = [
        '<path d="M4 7.5 12 3l8 4.5v9L12 21l-8-4.5v-9Zm0 0 8 4.5 8-4.5M12 12v9"/>',
        '<path d="M5 4h10l4 4v12H5V4Zm10 0v4h4M8 12h8M8 16h5"/>',
        '<path d="M5 6h5v5H5V6Zm9 7h5v5h-5v-5ZM10 8.5h4a2 2 0 0 1 2 2V13M8 11v5a2 2 0 0 0 2 2h4"/>',
        '<path d="M4 19V5M4 19h16M8 15l3-4 3 2 5-7"/>',
        '<path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm-7 8a7 7 0 0 1 14 0M18 7.5l2 1.5-2 1.5"/>'
      ];
      const coreServices = service.aiService.coreServices.map(function (item, index) {
        return `<article class="service-ai-core__item"><span class="service-ai-core__icon" aria-hidden="true"><svg viewBox="0 0 24 24">${aiCoreIcons[index]}</svg></span><div><h4>${item.title}</h4><ul>${item.items.map(function (detail) { return `<li>${detail}</li>`; }).join("")}</ul></div></article>`;
      }).join("");
      const advantages = service.aiService.advantages.map(function (item) {
        const description = item.description.includes("→")
          ? `<p class="service-ai-advantage__flow" aria-label="${item.description}">${item.description.split("→").map(function (step) { return `<span>${step}</span>`; }).join('<i aria-hidden="true">→</i>')}</p>`
          : `<p>${item.description}</p>`;
        return `<article><h4>${item.title}</h4>${description}</article>`;
      }).join("");
      const scenarios = service.aiService.scenarios.map(function (item) {
        return `<article><h4>${item.title}</h4><p>${item.description}</p></article>`;
      }).join("");
      const modes = service.aiService.modes.map(function (item) {
        return `<article><h4>${item.title}</h4><ul>${item.items.map(function (detail) { return `<li>${detail}</li>`; }).join("")}</ul><p>${item.suitable}</p></article>`;
      }).join("");
      aiSection.hidden = false;
      aiContent.innerHTML = `
        <header class="service-ai-intro">
          <h2>${service.aiService.title}</h2>
          <p class="service-ai-intro__lead">${service.aiService.lead}</p>
          <p class="service-ai-intro__slogan">${service.aiService.slogan}</p>
          <div>${service.aiService.description.map(function (paragraph) { return `<p>${paragraph}</p>`; }).join("")}</div>
        </header>
        <section class="service-ai-group" aria-labelledby="service-ai-core-title">
          <h3 id="service-ai-core-title">核心服务</h3>
          <div class="service-ai-core">${coreServices}</div>
        </section>
        <section class="service-ai-group" aria-labelledby="service-ai-advantages-title">
          <h3 id="service-ai-advantages-title">技术优势</h3>
          <div class="service-ai-advantages">${advantages}</div>
        </section>
        <section class="service-ai-group" aria-labelledby="service-ai-scenarios-title">
          <h3 id="service-ai-scenarios-title">应用场景</h3>
          <div class="service-ai-scenarios">${scenarios}</div>
        </section>
        <section class="service-ai-group" aria-labelledby="service-ai-modes-title">
          <h3 id="service-ai-modes-title">服务模式</h3>
          <div class="service-ai-modes">${modes}</div>
        </section>
      `;
    } else {
      aiSection.hidden = true;
      aiContent.innerHTML = "";
    }

    const cases = document.getElementById("service-profile-cases");
    const casesSection = document.getElementById("service-profile-cases-section");
    const casesToggle = document.getElementById("service-case-toggle");
    if (service.cases && service.cases.length) {
      casesSection.hidden = false;
      cases.innerHTML = service.cases.map(function (item, index) {
        return `<li${index >= 10 ? " hidden" : ""}>${item}</li>`;
      }).join("");
      if (service.cases.length > 10) {
        casesToggle.hidden = false;
        casesToggle.addEventListener("click", function () {
          const expanded = casesToggle.getAttribute("aria-expanded") === "true";
          casesToggle.setAttribute("aria-expanded", String(!expanded));
          cases.querySelectorAll("li").forEach(function (item, index) {
            if (index >= 10) item.hidden = expanded;
          });
          casesToggle.textContent = expanded ? "查看全部客户 +" : "收起客户 −";
        });
      }
    } else {
      casesSection.hidden = true;
      cases.innerHTML = "";
      casesToggle.hidden = true;
    }

    Array.from(document.querySelectorAll(".service-workflow-full")).forEach(function (details) {
      const summary = details.querySelector(":scope > summary");
      if (!summary) return;
      function syncDetailState() {
        summary.setAttribute("aria-expanded", String(details.open));
      }
      details.addEventListener("toggle", syncDetailState);
      syncDetailState();
    });

    const anchorItems = [
      ["service-offerings-section", isProductCatalog ? "软件产品" : "服务内容"],
      ["service-profile-technology-section", isProductCatalog ? "技术与服务能力" : "核心能力"],
      ["service-profile-workflow-section", "服务流程"]
    ];
    if (!aiSection.hidden) anchorItems.push(["service-profile-ai-section", "AI 服务"]);
    if (!casesSection.hidden) anchorItems.push(["service-profile-cases-section", isProductCatalog ? "典型客户" : "典型实践"]);
    anchorItems.push(["service-profile-contact-section", "服务咨询"]);
    const anchorLinks = document.getElementById("service-profile-anchor-links");
    anchorLinks.innerHTML = anchorItems.map(function (item, index) {
      return `<a class="${index === 0 ? "is-active" : ""}" href="#${item[0]}"${index === 0 ? " aria-current=\"location\"" : ""}>${item[1]}</a>`;
    }).join("");

    const pageAnchorLinks = Array.from(anchorLinks.querySelectorAll("a"));
    const anchorSections = anchorItems.map(function (item) { return document.getElementById(item[0]); });
    function setActiveAnchor(sectionId) {
      pageAnchorLinks.forEach(function (link) {
        const active = link.getAttribute("href") === `#${sectionId}`;
        link.classList.toggle("is-active", active);
        if (active) link.setAttribute("aria-current", "location");
        else link.removeAttribute("aria-current");
      });
    }
    function updateActiveAnchor() {
      const activationLine = document.getElementById("service-profile-anchor-nav").getBoundingClientRect().bottom + 12;
      let activeSection = anchorSections[0];
      anchorSections.forEach(function (section) {
        if (section.getBoundingClientRect().top <= activationLine) activeSection = section;
      });
      setActiveAnchor(activeSection.id);
    }
    let anchorFrame = 0;
    function requestAnchorUpdate() {
      if (anchorFrame) return;
      anchorFrame = window.requestAnimationFrame(function () {
        updateActiveAnchor();
        anchorFrame = 0;
      });
    }
    pageAnchorLinks.forEach(function (link) {
      link.addEventListener("click", function () {
        setActiveAnchor(link.getAttribute("href").slice(1));
      });
    });
    window.addEventListener("scroll", requestAnchorUpdate, { passive: true });
    window.addEventListener("resize", requestAnchorUpdate);
    if ("IntersectionObserver" in window) {
      const anchorObserver = new IntersectionObserver(requestAnchorUpdate, { rootMargin: "-150px 0px -55% 0px", threshold: [0, 0.15, 0.4] });
      anchorSections.forEach(function (section) { anchorObserver.observe(section); });
    }
    updateActiveAnchor();
  }

  const introduction = document.querySelector("[data-about-introduction]");
  if (introduction && generatedContent.introduction) {
    introduction.innerHTML = generatedContent.introduction;
  }

  const recruitmentList = document.querySelector("[data-recruitment-list]");
  const disclosureList = document.querySelector("[data-disclosure-list]");
  if (recruitmentList || disclosureList) {
    function renderAboutEntries(target, items, emptyText) {
      if (!target) return;
      target.replaceChildren();
      const latest = Array.isArray(items) ? items.slice().sort(function (a, b) {
        const dateOrder = String(b.date || "").localeCompare(String(a.date || ""));
        return dateOrder || Number(b.articleId || 0) - Number(a.articleId || 0);
      }).slice(0, 5) : [];
      if (!latest.length) {
        const row = document.createElement("li");
        const label = document.createElement("span");
        label.textContent = emptyText;
        row.append(label);
        target.append(row);
        return;
      }
      const rows = document.createDocumentFragment();
      latest.forEach(function (item) {
        const row = document.createElement("li");
        const label = document.createElement(item.href ? "a" : "span");
        label.textContent = item.title || "";
        if (item.href) {
          label.href = rootUrl(item.href);
          if (item.external) {
            label.target = "_blank";
            label.rel = "noopener noreferrer";
          }
        }
        row.append(label);
        if (item.date) {
          const date = document.createElement("time");
          date.dateTime = item.date;
          date.textContent = item.date;
          row.append(date);
        }
        rows.append(row);
      });
      target.append(rows);
    }
    renderAboutEntries(recruitmentList, generatedContent.recruitment, "暂无招聘信息");
    renderAboutEntries(disclosureList, generatedContent.disclosure, "暂无信息公开内容");
  }

  const newsList = document.querySelector("[data-news-list]");
  const filterButtons = document.querySelectorAll("[data-filter]");

  if (newsList) {
    const newsCards = Array.from(newsList.querySelectorAll("[data-category]"));
    const loadMore = document.querySelector("[data-news-load-more]");
    const loadMoreButton = loadMore.querySelector("button");
    const newsCount = loadMore.querySelector("[data-news-count]");
    const initialVisibleCount = 5;
    const loadCount = 2;
    let activeFilter = filterButtons.length ? filterButtons[0].dataset.filter : "";
    let visibleCount = initialVisibleCount;

    function renderNews() {
      const matchingCards = newsCards.filter(function (card) {
        return !activeFilter || card.dataset.category === activeFilter;
      });

      newsCards.forEach(function (card) {
        const matchingIndex = matchingCards.indexOf(card);
        card.classList.toggle("hidden", matchingIndex === -1 || matchingIndex >= visibleCount);
        card.classList.remove("news-card--feature");
      });
      if (matchingCards.length) matchingCards[0].classList.add("news-card--feature");

      const displayedCount = Math.min(visibleCount, matchingCards.length);
      const hasMore = displayedCount < matchingCards.length;
      loadMoreButton.hidden = !hasMore;
      newsCount.textContent = hasMore
        ? `已显示 ${displayedCount} / ${matchingCards.length} 条`
        : `已显示全部 ${matchingCards.length} 条`;
    }

    filterButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        activeFilter = button.dataset.filter;
        visibleCount = initialVisibleCount;
        filterButtons.forEach(function (item) {
          item.classList.toggle("is-active", item === button);
        });
        renderNews();
      });
    });

    loadMoreButton.addEventListener("click", function () {
      visibleCount += loadCount;
      renderNews();
    });

    renderNews();
  } else {
    filterButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        const filter = button.dataset.filter;
        filterButtons.forEach(function (item) {
          item.classList.toggle("is-active", item === button);
        });
        document.querySelectorAll("[data-category]").forEach(function (item) {
          item.classList.toggle("hidden", filter !== "全部" && item.dataset.category !== filter);
        });
      });
    });
  }
})();
