(function initAssociationWorkPage() {
  "use strict";

  document.querySelectorAll(".work-tabs").forEach(function bindTabGroup(tabList) {
    var buttons = Array.prototype.slice.call(tabList.querySelectorAll("[data-work-tab]"));

    function activateTab(button) {
      buttons.forEach(function updateButton(item) {
        var panel = document.getElementById(item.getAttribute("data-work-tab"));
        var isActive = item === button;

        item.classList.toggle("active", isActive);
        item.setAttribute("aria-selected", String(isActive));
        if (panel) {
          panel.classList.toggle("active", isActive);
          panel.hidden = !isActive;
        }
      });
    }

    buttons.forEach(function bindButton(button, index) {
      button.addEventListener("mouseenter", function onTabHover() {
        activateTab(button);
      });
      button.addEventListener("click", function onTabClick() {
        activateTab(button);
      });
      button.addEventListener("keydown", function onTabKeydown(event) {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        var offset = event.key === "ArrowRight" ? 1 : -1;
        var next = buttons[(index + offset + buttons.length) % buttons.length];
        next.focus();
        activateTab(next);
      });
    });
  });

  var category = new URLSearchParams(window.location.search).get("category");
  var categoryPanels = {
    "work-file": "association-panel",
    "work-brand": "association-panel",
    "work-industry": "industry-panel",
    "work-smart": "industry-panel",
    "work-expo": "expo-panel",
    "work-standard": "industry-panel"
  };
  var initialPanel = categoryPanels[category];
  var initialTab = initialPanel ? document.querySelector('[data-work-tab="' + initialPanel + '"]') : null;

  if (initialTab) initialTab.dispatchEvent(new Event("mouseenter"));

  document.querySelectorAll("[data-footer-link]").forEach(function bindFooterLink(select) {
    select.addEventListener("change", function openFooterLink() {
      if (select.value) window.open(select.value, "_blank", "noopener,noreferrer");
      select.selectedIndex = 0;
    });
  });
})();
