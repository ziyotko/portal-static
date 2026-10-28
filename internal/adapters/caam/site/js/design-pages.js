(function () {
  "use strict";

  function initTabs(tabSelector, panelSelector) {
    var tabs = Array.prototype.slice.call(document.querySelectorAll(tabSelector));
    var panels = Array.prototype.slice.call(document.querySelectorAll(panelSelector));

    function activateTab(tab, moveFocus) {
      var target = tab.getAttribute("aria-controls");
      tabs.forEach(function (item) {
        var selected = item === tab;
        item.classList.toggle("active", selected);
        item.setAttribute("aria-selected", String(selected));
        item.setAttribute("tabindex", selected ? "0" : "-1");
      });
      panels.forEach(function (panel) {
        panel.hidden = panel.id !== target;
      });
      if (moveFocus) {
        tab.focus();
      }
    }

    tabs.forEach(function (tab, index) {
      tab.addEventListener("click", function (event) {
        event.preventDefault();
        activateTab(tab, false);
      });
      tab.addEventListener("keydown", function (event) {
        var nextIndex = index;
        if (event.key === "ArrowRight") nextIndex = (index + 1) % tabs.length;
        else if (event.key === "ArrowLeft") nextIndex = (index - 1 + tabs.length) % tabs.length;
        else if (event.key === "Home") nextIndex = 0;
        else if (event.key === "End") nextIndex = tabs.length - 1;
        else return;
        event.preventDefault();
        activateTab(tabs[nextIndex], true);
      });
    });

    var selectedTab = tabs.find(function (tab) {
      return tab.getAttribute("aria-selected") === "true";
    }) || tabs[0];
    if (selectedTab) activateTab(selectedTab, false);
  }

  function initFooterLinks() {
    document.querySelectorAll("[data-footer-link]").forEach(function (select) {
      select.addEventListener("change", function () {
        if (/^https?:\/\//i.test(select.value)) {
          window.open(select.value, "_blank", "noopener,noreferrer");
        }
        select.selectedIndex = 0;
      });
    });
  }

  function initPartyBanner() {
    document.querySelectorAll("[data-party-banner]").forEach(function (banner) {
      var slides = Array.prototype.slice.call(banner.querySelectorAll(".party-feature-slide"));
      var dots = Array.prototype.slice.call(banner.querySelectorAll(".party-banner-dots button"));

      function showSlide(index) {
        slides.forEach(function (slide, slideIndex) {
          slide.classList.toggle("active", slideIndex === index);
        });
        dots.forEach(function (dot, dotIndex) {
          var active = dotIndex === index;
          dot.classList.toggle("active", active);
          if (active) {
            dot.setAttribute("aria-current", "true");
          } else {
            dot.removeAttribute("aria-current");
          }
        });
      }

      dots.forEach(function (dot, index) {
        dot.addEventListener("click", function () {
          showSlide(index);
        });
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    initTabs("[data-member-tab]", ".member-tab-panel");
    initTabs("[data-roster-tab]", ".roster-panel");
    initPartyBanner();
    initFooterLinks();
  });
})();
