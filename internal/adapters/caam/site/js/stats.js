(function initStatisticsPage() {
  "use strict";

  document.querySelectorAll(".data-tabs").forEach(function bindTabList(tabList) {
    var tabs = Array.prototype.slice.call(tabList.querySelectorAll("[data-stat-tab]"));

    function activateTab(tab) {
      tabs.forEach(function updateTab(item) {
        var panel = document.getElementById(item.getAttribute("data-stat-tab"));
        var active = item === tab;
        item.classList.toggle("active", active);
        item.setAttribute("aria-selected", String(active));
        item.tabIndex = active ? 0 : -1;
        if (panel) {
          panel.classList.toggle("active", active);
          panel.hidden = !active;
        }
      });
    }

    tabs.forEach(function bindTab(tab, index) {
      tab.addEventListener("mouseenter", function onTabHover() {
        activateTab(tab);
      });
      tab.addEventListener("click", function onTabClick() {
        activateTab(tab);
      });
      tab.addEventListener("keydown", function onTabKeydown(event) {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        var direction = event.key === "ArrowRight" ? 1 : -1;
        var next = tabs[(index + direction + tabs.length) % tabs.length];
        activateTab(next);
        next.focus();
      });
    });

    activateTab(tabList.querySelector(".active") || tabs[0]);
  });

  document.querySelectorAll("[data-footer-link]").forEach(function bindFooterLink(select) {
    select.addEventListener("change", function openFooterLink() {
      if (select.value) window.open(select.value, "_blank", "noopener,noreferrer");
      select.selectedIndex = 0;
    });
  });
})();
