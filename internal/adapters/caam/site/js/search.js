(function initSearchPage() {
  "use strict";

  var searchEndpoint = "/caamm/api/search/articles";
  var pageSize = 8;
  var input = document.getElementById("site-search-input");
  var form = document.querySelector(".site-search-form");
  var summary = document.getElementById("search-summary");
  var count = document.getElementById("search-count");
  var list = document.getElementById("search-result-list");
  var empty = document.getElementById("search-empty");
  var pagination = document.getElementById("search-pagination");
  var params = new URLSearchParams(window.location.search);
  var activeRequest = null;
  var requestSequence = 0;
  var state = {
    query: (params.get("q") || "").trim(),
    page: Math.max(1, Number(params.get("page")) || 1)
  };

  function appendHighlighted(element, text, keyword) {
    var source = String(text || "");
    var needle = String(keyword || "").trim();
    var lowerSource;
    var lowerNeedle;
    var start = 0;
    var index;
    if (!needle) {
      element.textContent = source;
      return;
    }
    lowerSource = source.toLocaleLowerCase();
    lowerNeedle = needle.toLocaleLowerCase();
    index = lowerSource.indexOf(lowerNeedle, start);
    while (index !== -1) {
      if (index > start) element.appendChild(document.createTextNode(source.slice(start, index)));
      var mark = document.createElement("mark");
      mark.textContent = source.slice(index, index + needle.length);
      element.appendChild(mark);
      start = index + needle.length;
      index = lowerSource.indexOf(lowerNeedle, start);
    }
    if (start < source.length) element.appendChild(document.createTextNode(source.slice(start)));
  }

  function firstDefined(source, keys, fallback) {
    if (!source || typeof source !== "object") return fallback;
    for (var index = 0; index < keys.length; index += 1) {
      var value = source[keys[index]];
      if (value !== undefined && value !== null && value !== "") return value;
    }
    return fallback;
  }

  function firstNumber(source, keys, fallback) {
    var value = Number(firstDefined(source, keys, NaN));
    return Number.isFinite(value) && value >= 0 ? value : fallback;
  }

  function plainText(value) {
    var holder = document.createElement("div");
    holder.innerHTML = String(value || "");
    return (holder.textContent || "").replace(/\s+/g, " ").trim();
  }

  function dateParts(value) {
    var raw = String(value || "").trim();
    var matched = raw.match(/^(\d{4})[-\/]([01]?\d)[-\/]([0-3]?\d)/);
    var parsed;
    if (matched) {
      return {
        iso: matched[1] + "-" + matched[2].padStart(2, "0") + "-" + matched[3].padStart(2, "0"),
        year: matched[1],
        month: matched[2].padStart(2, "0"),
        day: matched[3].padStart(2, "0")
      };
    }
    if (/^\d+$/.test(raw)) parsed = new Date(Number(raw) < 1000000000000 ? Number(raw) * 1000 : Number(raw));
    else parsed = new Date(raw.replace(" ", "T"));
    if (Number.isNaN(parsed.getTime())) return null;
    return {
      iso: parsed.getFullYear() + "-" + String(parsed.getMonth() + 1).padStart(2, "0") + "-" + String(parsed.getDate()).padStart(2, "0"),
      year: String(parsed.getFullYear()),
      month: String(parsed.getMonth() + 1).padStart(2, "0"),
      day: String(parsed.getDate()).padStart(2, "0")
    };
  }

  function formatDate(parts) {
    return parts ? parts.year + "年" + Number(parts.month) + "月" + Number(parts.day) + "日" : "";
  }

  function safeHref(value) {
    var raw = String(value || "").trim();
    if (!raw || raw.indexOf("//") === 0 || raw.indexOf("\\") !== -1) return "";
    try {
      var parsed = new URL(raw, window.location.href);
      return parsed.protocol === "http:" || parsed.protocol === "https:" ? raw : "";
    } catch (error) {
      return "";
    }
  }

  function articleHref(source, articleID, published) {
    var checkedHref = safeHref(firstDefined(source, ["href", "url", "link", "articleUrl"], ""));
    if (/^https?:\/\//i.test(checkedHref)) return checkedHref;
    if (articleID && published) {
      var origin = new URLSearchParams({ from: "search", title: state.query, page: String(state.page) });
      return "article/" + published.year + "/" + published.month + "/" + encodeURIComponent(articleID) + ".html?" + origin.toString();
    }
    return checkedHref || "#";
  }

  function normalizeArticle(source) {
    var articleID = String(firstDefined(source, ["id", "articleId", "articleID"], "")).trim();
    var published = dateParts(firstDefined(source, ["publishTime", "publishDate", "publishedAt", "date", "createTime"], ""));
    var column = firstDefined(source, ["column", "category"], null);
    var category = firstDefined(source, ["columnName", "categoryName", "channelName", "typeName"], "");
    if (!category && column && typeof column === "object") category = firstDefined(column, ["name", "title"], "");
    if (!category && typeof column === "string") category = column;
    return {
      title: plainText(firstDefined(source, ["title", "articleTitle", "name"], "未命名内容")),
      category: plainText(category),
      summary: plainText(firstDefined(source, ["summary", "description", "abstract", "introduction", "content"], "")),
      date: published,
      href: articleHref(source, articleID, published)
    };
  }

  function unwrapPayload(payload) {
    var body = payload;
    for (var depth = 0; depth < 3; depth += 1) {
      if (!body || Array.isArray(body) || typeof body !== "object") break;
      if (["records", "list", "content", "rows", "items", "data"].some(function hasList(key) { return Array.isArray(body[key]); })) break;
      if (body.data !== undefined && body.data !== null) body = body.data;
      else if (body.result !== undefined && body.result !== null) body = body.result;
      else break;
    }
    return body;
  }

  function normalizeResponse(payload) {
    var body = unwrapPayload(payload);
    var items;
    var total;
    var totalPages;
    if (Array.isArray(body)) {
      items = body;
      total = body.length;
      totalPages = Math.max(1, Math.ceil(total / pageSize));
    } else {
      body = body && typeof body === "object" ? body : {};
      items = firstDefined(body, ["records", "list", "content", "rows", "items", "data"], []);
      if (!Array.isArray(items)) items = [];
      total = firstNumber(body, ["total", "totalElements", "totalCount", "count"], items.length);
      totalPages = firstNumber(body, ["pages", "totalPages", "pageCount"], Math.max(1, Math.ceil(total / pageSize)));
    }
    return { items: items.map(normalizeArticle), total: total, totalPages: Math.max(1, totalPages) };
  }

  function updateURL() {
    var next = new URLSearchParams();
    if (state.query) next.set("q", state.query);
    if (state.page > 1) next.set("page", String(state.page));
    history.replaceState(null, "", "search.html" + (next.toString() ? "?" + next.toString() : ""));
  }

  function hideResults() {
    count.hidden = true;
    list.hidden = true;
    pagination.hidden = true;
    list.innerHTML = "";
    pagination.innerHTML = "";
  }

  function showMessage(title, description) {
    hideResults();
    empty.hidden = false;
    empty.querySelector("strong").textContent = title;
    empty.querySelector("p").textContent = description;
  }

  function renderPagination(totalPages) {
    pagination.innerHTML = "";
    pagination.hidden = totalPages <= 1;
    if (totalPages <= 1) return;

    function addButton(label, page, disabled, active, ariaLabel) {
      var button = document.createElement("button");
      button.type = "button";
      button.textContent = label;
      button.disabled = disabled;
      if (active) {
        button.className = "active";
        button.setAttribute("aria-current", "page");
      }
      if (ariaLabel) button.setAttribute("aria-label", ariaLabel);
      button.addEventListener("click", function changePage() {
        state.page = page;
        updateURL();
        loadResults(true);
      });
      pagination.appendChild(button);
    }

    function addEllipsis() {
      var ellipsis = document.createElement("span");
      ellipsis.className = "ellipsis";
      ellipsis.textContent = "…";
      ellipsis.setAttribute("aria-hidden", "true");
      pagination.appendChild(ellipsis);
    }

    addButton("‹", Math.max(1, state.page - 1), state.page === 1, false, "上一页");
    var visiblePages = [];
    for (var page = 1; page <= totalPages; page += 1) {
      if (page === 1 || page === totalPages || (page >= state.page - 2 && page <= state.page + 2)) visiblePages.push(page);
    }
    visiblePages.forEach(function addPage(page, index) {
      if (index > 0 && page - visiblePages[index - 1] > 1) addEllipsis();
      addButton(String(page), page, false, page === state.page, "第" + page + "页");
    });
    addButton("›", Math.min(totalPages, state.page + 1), state.page === totalPages, false, "下一页");
  }

  function renderResults(result) {
    list.innerHTML = "";
    result.items.forEach(function renderItem(item) {
      var row = document.createElement("li");
      var main = document.createElement("div");
      var titleRow = document.createElement("div");
      var link = document.createElement("a");
      row.className = "search-result-item";
      main.className = "search-result-main";
      titleRow.className = "search-result-title-row";
      link.href = item.href;
      appendHighlighted(link, item.title, state.query);
      titleRow.appendChild(link);
      if (item.category) {
        var tag = document.createElement("span");
        tag.className = "search-result-tag";
        tag.textContent = item.category;
        titleRow.appendChild(tag);
      }
      main.appendChild(titleRow);
      if (item.summary) {
        var description = document.createElement("p");
        appendHighlighted(description, item.summary, state.query);
        main.appendChild(description);
      }
      row.appendChild(main);
      if (item.date) {
        var time = document.createElement("time");
        time.dateTime = item.date.iso;
        time.textContent = formatDate(item.date);
        row.appendChild(time);
      }
      list.appendChild(row);
    });
    empty.hidden = true;
    list.hidden = false;
    count.hidden = false;
    count.querySelector("strong").textContent = String(result.total);
    renderPagination(result.totalPages);
  }

  function setLoading() {
    hideResults();
    empty.hidden = false;
    empty.querySelector("strong").textContent = "正在搜索…";
    empty.querySelector("p").textContent = "正在获取与“" + state.query + "”相关的内容";
  }

  function loadResults(scrollToResults) {
    input.value = state.query;
    if (!state.query) {
      summary.textContent = "请输入关键词开始搜索";
      showMessage("输入关键词，查找您需要的内容", "支持按文章标题进行检索");
      return;
    }

    var sequence = requestSequence + 1;
    requestSequence = sequence;
    if (activeRequest) activeRequest.abort();
    activeRequest = typeof AbortController === "function" ? new AbortController() : null;
    summary.textContent = "正在搜索“" + state.query + "”";
    setLoading();

    var query = new URLSearchParams({ title: state.query, page: String(state.page), pageSize: String(pageSize) });
    var options = { method: "GET", headers: { Accept: "application/json" }, credentials: "same-origin" };
    if (activeRequest) options.signal = activeRequest.signal;

    fetch(searchEndpoint + "?" + query.toString(), options)
      .then(function checkResponse(response) {
        if (!response.ok) throw new Error("search request failed with status " + response.status);
        return response.json();
      })
      .then(function handleResponse(payload) {
        if (sequence !== requestSequence) return;
        var result = normalizeResponse(payload);
        if (state.page > result.totalPages) {
          state.page = result.totalPages;
          updateURL();
          loadResults(scrollToResults);
          return;
        }
        summary.textContent = "与“" + state.query + "”相关的内容";
        if (!result.items.length) {
          showMessage("未找到相关内容", "请尝试缩短关键词或更换搜索词");
          return;
        }
        renderResults(result);
        if (scrollToResults) document.querySelector(".search-results-card").scrollIntoView({ behavior: "smooth", block: "start" });
      })
      .catch(function handleError(error) {
        if (error && error.name === "AbortError") return;
        if (sequence !== requestSequence) return;
        summary.textContent = "搜索服务暂时不可用";
        showMessage("搜索失败，请稍后重试", "未能连接站内搜索服务");
      });
  }

  form.addEventListener("submit", function submitSearch(event) {
    event.preventDefault();
    var query = input.value.trim();
    if (!query) {
      input.focus();
      return;
    }
    state.query = query;
    state.page = 1;
    updateURL();
    loadResults(false);
  });

  document.querySelectorAll("[data-search-keyword]").forEach(function bindHotKeyword(button) {
    button.addEventListener("click", function searchKeyword() {
      state.query = button.dataset.searchKeyword;
      state.page = 1;
      updateURL();
      loadResults(false);
    });
  });

  document.querySelectorAll("[data-footer-link]").forEach(function bindFooterLink(select) {
    select.addEventListener("change", function openFooterLink() {
      if (select.value) window.open(select.value, "_blank", "noopener,noreferrer");
      select.selectedIndex = 0;
    });
  });

  loadResults(false);
})();
