(function initGeneratedInnerPages() {
  "use strict";

  document.querySelectorAll("[data-footer-link]").forEach(function bindFooterLink(select) {
    select.addEventListener("change", function openFooterLink() {
      if (select.value) window.open(select.value, "_blank", "noopener,noreferrer");
      select.selectedIndex = 0;
    });
  });

  var detailMain = document.querySelector("[data-detail-main]");
  if (!detailMain) return;

  var rootPrefix = detailMain.getAttribute("data-root-prefix") || "../../../";
  var params = new URLSearchParams(window.location.search);
  var source = params.get("from");
  var returnLink = document.querySelector("[data-return-link]");
  var columnCrumb = document.querySelector("[data-column-crumb]");
  var columnLink = document.querySelector("[data-column-link]");
  var fallbackHref = rootPrefix + "index.html";

  if (source === "list") {
    var columnID = params.get("column_id") || "";
    var page = params.get("page") || "1";
    var columnTitle = (params.get("column_title") || "栏目列表").trim();
    if (/^\d+$/.test(columnID) && /^\d+$/.test(page)) {
      fallbackHref = rootPrefix + "list/" + columnID + "/" + page + ".html";
      if (columnCrumb && columnLink) {
        columnLink.href = fallbackHref;
        columnLink.textContent = columnTitle || "栏目列表";
        columnCrumb.hidden = false;
      }
    }
  }

  if (source === "search") {
    var searchQuery = (params.get("title") || "").trim();
    var searchPage = params.get("page") || "1";
    var searchParams = new URLSearchParams();
    if (searchQuery) searchParams.set("q", searchQuery);
    if (/^\d+$/.test(searchPage) && searchPage !== "1") searchParams.set("page", searchPage);
    fallbackHref = rootPrefix + "search.html" + (searchParams.toString() ? "?" + searchParams.toString() : "");
  }

  if (source === "about") {
    var aboutSection = (params.get("section") || "").trim();
    fallbackHref = rootPrefix + "about.html" + (/^[a-z-]+$/.test(aboutSection) ? "#" + aboutSection : "");
    if (columnCrumb && columnLink) {
      columnLink.href = fallbackHref;
      columnLink.textContent = "协会概况";
      columnCrumb.hidden = false;
    }
  }

  if (source === "work" || source === "stats") {
    var sourceSection = (params.get("section") || "").trim();
    var sourceTitle = source === "work" ? "协会工作" : "统计数据";
    var sourcePage = source === "work" ? "work.html" : "stats.html";
    fallbackHref = rootPrefix + sourcePage + (/^[a-z-]+$/.test(sourceSection) ? "#" + sourceSection : "");
    if (columnCrumb && columnLink) {
      columnLink.href = fallbackHref;
      columnLink.textContent = sourceTitle;
      columnCrumb.hidden = false;
    }
  }

  if (source === "members" || source === "party") {
    var channelSection = (params.get("section") || "").trim();
    var channelTitle = source === "members" ? "会员专区" : "党建专区";
    var channelPage = source === "members" ? "members.html" : "party.html";
    fallbackHref = rootPrefix + channelPage + (/^[a-z-]+$/.test(channelSection) ? "#" + channelSection : "");
    if (columnCrumb && columnLink) {
      columnLink.href = fallbackHref;
      columnLink.textContent = channelTitle;
      columnCrumb.hidden = false;
    }
  }

  if (returnLink) {
    returnLink.href = fallbackHref;
    returnLink.addEventListener("click", function returnToSource(event) {
      if (document.referrer && window.history.length > 1) {
        event.preventDefault();
        window.history.back();
      }
    });
  }
})();
