(function initInnerPages() {
  "use strict";

  var categoryCatalog = {
    "association": {
      title: "协会动态",
      items: ["产教深度融合 赋能产业升级中汽协职业技术分会第七期交流会在芜湖顺利举行", "中国汽车工业协会会员服务交流会成功举办", "汽车产业人才培养与技术创新协同发展研讨会在京召开"]
    },
    "association-activity": {
      title: "协会活动",
      items: ["《中国汽车电驱动产业发展研究报告（2026版）》启动会在京召开", "中国汽车工业协会会员服务交流会成功举办", "汽车产业人才培养专题研讨会在芜湖召开"]
    },
    "work-file": {
      title: "协会文件",
      items: ["《2025中国汽车后市场年度发展报告》正式发布", "中国汽车工业协会关于汽车行业绿色发展的倡议", "关于开展会员企业服务需求调研的通知"]
    },
    "work-industry": {
      title: "行业发展",
      items: ["汽车产业运行总体平稳，新动能持续增强", "新能源与智能网联融合发展专题研究启动", "汽车后市场服务体系建设研讨会召开"]
    },
    "work-smart": {
      title: "智能网联",
      items: ["智能网联汽车准入和上路通行试点稳步推进", "车路云一体化应用场景加速落地", "汽车数据安全与合规治理专题会议召开"]
    },
    "work-brand": {
      title: "品牌服务",
      items: ["中国汽车品牌向上发展专项行动正式启动", "会员企业品牌建设经验交流会成功举办", "汽车品牌国际传播能力建设研讨会召开"]
    },
    "work-expo": {
      title: "展会信息",
      items: ["2026中国汽车供应链大会筹备工作全面启动", "香港国际汽车及供应链博览会即将举办", "中国汽车论坛专题展区开放报名"]
    },
    "work-standard": {
      title: "标准法规",
      items: ["汽车行业标准体系建设工作会议召开", "公开征求智能网联汽车相关标准意见", "新能源汽车安全技术标准研讨会举办"]
    },
    "industry-news": {
      title: "行业要闻",
      items: ["2026年新能源汽车下乡活动在两地同步启动", "汽车产业加快形成绿色低碳发展新格局", "我国汽车出口市场延续稳健增长态势"]
    },
    "notice": {
      title: "通知公告",
      items: ["关于征求2026年新能源汽车下乡活动推荐车型意见的通知", "关于开展汽车行业数字化转型案例征集的通知", "2026中国汽车论坛参会报名正式开启"]
    },
    "company-news": {
      title: "企业新闻",
      items: ["中国汽车品牌持续加快全球化布局", "车企联合产业伙伴推进低碳供应链建设", "新一代智能电动平台集中亮相"]
    },
    "international-cooperation": {
      title: "国际合作",
      items: ["中国汽车工业协会代表团赴欧洲开展行业交流", "中外汽车产业合作交流会在北京举行", "汽车标准国际协调工作取得新进展"]
    },
    "industry-policy": {
      title: "行业政策",
      items: ["多项政策协同促进汽车消费提质升级", "新能源汽车下乡支持政策持续优化", "智能网联汽车试点政策解读发布"]
    },
    "laws-regulations": {
      title: "法律法规",
      items: ["汽车产品安全与召回管理制度持续完善", "道路机动车辆生产准入管理要求公开征求意见", "汽车数据跨境合规指引发布"]
    }
  };
  var categoryFillers = ["重点工作取得新进展", "专题交流活动顺利举行", "行业调研工作正式启动", "创新实践案例集中发布", "服务能力建设持续推进", "年度工作会议在京召开", "高质量发展行动稳步实施"];

  function getCategory(category) {
    return categoryCatalog[category] || categoryCatalog.association;
  }

  function isWorkCategory(category) {
    return category === "association" || category === "association-activity" || category.indexOf("work-") === 0;
  }

  function getCategoryTitles(category) {
    var config = getCategory(category);
    var titles = config.items.slice();
    categoryFillers.forEach(function addFiller(text) {
      if (titles.length < 10) titles.push(config.title + "：" + text);
    });
    return titles;
  }

  if (document.body.classList.contains("content-list-page")) {
    var listParams = new URLSearchParams(window.location.search);
    var listCategoryKey = listParams.get("category") || "association";
    var listCategory = getCategory(listCategoryKey);
    var listTitle = document.getElementById("list-title");
    var breadcrumbTitle = document.getElementById("list-breadcrumb-title");
    var newsList = document.getElementById("category-news-list");

    if (listTitle) listTitle.textContent = listCategory.title;
    if (breadcrumbTitle) breadcrumbTitle.textContent = listCategory.title;
    document.title = listCategory.title + " - 中国汽车工业协会";

    if (newsList && listCategoryKey !== "association") {
      newsList.innerHTML = "";
      getCategoryTitles(listCategoryKey).forEach(function addCategoryItem(title, index) {
        var item = document.createElement("li");
        var link = document.createElement("a");
        var time = document.createElement("time");
        var day = 27 - index;
        link.href = "article.html?id=" + encodeURIComponent(listCategoryKey + "-" + (index + 1)) + "&category=" + encodeURIComponent(listCategoryKey);
        link.textContent = title;
        time.dateTime = "2026-05-" + String(day).padStart(2, "0");
        time.textContent = "2026年5月" + day + "日";
        item.appendChild(link);
        item.appendChild(time);
        newsList.appendChild(item);
      });
    }
  }

  if (document.body.classList.contains("content-detail-page")) {
    var detailParams = new URLSearchParams(window.location.search);
    var detailCategoryKey = detailParams.get("category");
    var backLink = document.querySelector(".back-link");
    if (detailParams.get("from") === "home") {
      document.querySelectorAll(".detail-context-crumb").forEach(function hideDetailContext(item) {
        item.hidden = true;
      });
      if (backLink) {
        backLink.href = "index.html";
        backLink.innerHTML = "<span>‹</span> 返回首页";
      }
    } else if (detailCategoryKey) {
      var detailCategory = getCategory(detailCategoryKey);
      var detailCategoryLink = document.getElementById("detail-category-link");
      var detailTitles = getCategoryTitles(detailCategoryKey);
      var detailID = detailParams.get("id") || "";
      var detailIndex = Number(detailID.split("-").pop()) - 1;

      if (backLink) backLink.href = "list.html?category=" + encodeURIComponent(detailCategoryKey);
      if (detailCategoryLink) {
        detailCategoryLink.href = "list.html?category=" + encodeURIComponent(detailCategoryKey);
        detailCategoryLink.textContent = detailCategory.title;
      }
      var articleCategoryMeta = document.getElementById("article-category-meta");
      if (articleCategoryMeta) articleCategoryMeta.textContent = "分类：" + detailCategory.title;
      if (!isWorkCategory(detailCategoryKey)) {
        document.querySelectorAll(".detail-work-crumb").forEach(function hideWorkContext(item) {
          item.hidden = true;
        });
      }
      if (detailIndex >= 0 && detailTitles[detailIndex]) {
        var articleTitle = document.querySelector(".article-header h1");
        if (articleTitle) articleTitle.textContent = detailTitles[detailIndex];
        document.title = detailTitles[detailIndex] + " - 中国汽车工业协会";
      }
    }
  }

  var pageButtons = Array.prototype.slice.call(document.querySelectorAll(".page-number"));
  pageButtons.forEach(function bindPageButton(button) {
    button.addEventListener("click", function onPageClick() {
      pageButtons.forEach(function updatePage(item) {
        var active = item === button;
        item.classList.toggle("active", active);
        if (active) item.setAttribute("aria-current", "page");
        else item.removeAttribute("aria-current");
      });
    });
  });

  document.querySelectorAll("[data-footer-link]").forEach(function bindFooterLink(select) {
    select.addEventListener("change", function openFooterLink() {
      if (select.value) window.open(select.value, "_blank", "noopener,noreferrer");
      select.selectedIndex = 0;
    });
  });
})();
