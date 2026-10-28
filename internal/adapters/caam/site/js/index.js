(function () {
  "use strict";

  function getItems(selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  }

  function setActive(items, activeIndex) {
    items.forEach(function updateItem(item, index) {
      item.classList.toggle("active", index === activeIndex);
    });
  }

  function initBanner() {
    var banner = document.querySelector("[data-banner]");
    if (!banner) {
      return;
    }

    var slides = getItems(".banner-slide", banner);
    var dots = getItems(".banner-dots button", banner);
    var prev = banner.querySelector(".banner-arrow-prev");
    var next = banner.querySelector(".banner-arrow-next");
    var current = 0;
    var timer = null;

    if (!slides.length) {
      return;
    }

    function showSlide(index) {
      current = (index + slides.length) % slides.length;
      setActive(slides, current);
      setActive(dots, current);
    }

    function showNext() {
      showSlide(current + 1);
    }

    function showPrev() {
      showSlide(current - 1);
    }

    function stopAutoPlay() {
      if (timer) {
        window.clearInterval(timer);
        timer = null;
      }
    }

    function startAutoPlay() {
      stopAutoPlay();
      if (slides.length < 2) {
        return;
      }
      timer = window.setInterval(showNext, 5000);
    }

    dots.forEach(function bindDot(dot, index) {
      dot.addEventListener("click", function onDotClick() {
        showSlide(index);
        startAutoPlay();
      });
    });

    if (prev) {
      prev.addEventListener("click", function onPrevClick() {
        showPrev();
        startAutoPlay();
      });
    }

    if (next) {
      next.addEventListener("click", function onNextClick() {
        showNext();
        startAutoPlay();
      });
    }

    banner.addEventListener("mouseenter", stopAutoPlay);
    banner.addEventListener("mouseleave", startAutoPlay);
    showSlide(0);
    startAutoPlay();
  }

  function initTabs() {
    var tabGroups = getItems("[data-tab-group]");

    tabGroups.forEach(function bindTabGroup(group) {
      var groupName = group.getAttribute("data-tab-group");
      var buttons = getItems("[data-tab-target]", group);
      var panels = getItems('[data-tab-panel="' + groupName + '"]');

      buttons.forEach(function bindTab(button) {
        button.addEventListener("click", function onTabClick() {
          var target = button.getAttribute("data-tab-target");

          buttons.forEach(function updateButton(item) {
            item.classList.toggle("active", item === button);
          });

          panels.forEach(function updatePanel(panel) {
            panel.classList.toggle("active", panel.id === target);
          });
        });
      });
    });
  }

  function initMenu() {
    var links = getItems("[data-work-target]");
    var panels = getItems(".work-list");
    var photo = document.querySelector("[data-work-photo]");

    function showWorkPanel(link) {
      var target = link.getAttribute("data-work-target");

      links.forEach(function updateLink(item) {
        item.classList.toggle("active", item === link);
      });

      panels.forEach(function updatePanel(panel) {
        panel.classList.toggle("active", panel.id === target);
      });

      if (photo && link.getAttribute("data-work-cover")) {
        photo.setAttribute("src", link.getAttribute("data-work-cover"));
        photo.setAttribute("alt", link.getAttribute("data-work-cover-alt") || link.textContent.trim() + "栏目封面");
      }
    }

    links.forEach(function bindWorkLink(link) {
      link.addEventListener("mouseenter", function onWorkEnter() {
        showWorkPanel(link);
      });
      link.addEventListener("focus", function onWorkFocus() {
        showWorkPanel(link);
      });
    });
  }

  function initIndustryMenu() {
    var links = getItems("[data-industry-tab]");
    var panels = getItems("[data-industry-panel]");

    links.forEach(function bindIndustryLink(link) {
      function activateLink() {
        var target = link.getAttribute("data-industry-tab");

        links.forEach(function updateLink(item) {
          item.classList.toggle("active", item === link);
        });

        panels.forEach(function updatePanel(panel) {
          panel.classList.toggle("active", panel.getAttribute("data-industry-panel") === target);
        });
      }

      link.addEventListener("mouseenter", activateLink);
      link.addEventListener("focus", activateLink);
    });
  }

  function initStatsNews() {
    var controls = getItems("[data-stats-news]");
    var panel = document.querySelector("#statistics-local");

    if (!controls.length || !panel) {
      return;
    }

    var label = panel.querySelector(".chart-label");
    var legends = getItems(".stats-legend span", panel);
    var numbers = getItems(".numbers b", panel);
    var bars = getItems(".bar-chart li", panel);

    function updateChart(control) {
      var valuesA = (control.getAttribute("data-bars-a") || "").split(",");
      var valuesB = (control.getAttribute("data-bars-b") || "").split(",");
      var unit = control.getAttribute("data-unit") || "万辆";
      var reportMonth = Number(control.getAttribute("data-report-month") || 0);

      controls.forEach(function updateControl(item) {
        var active = item === control;
        item.classList.toggle("active", active);
        item.setAttribute("aria-pressed", active ? "true" : "false");
      });

      if (label && label.firstChild) {
        label.firstChild.nodeValue = control.getAttribute("data-label") + " ";
      }

      if (label) {
        var unitLabel = label.querySelector("span");
        if (unitLabel) {
          unitLabel.textContent = "单位/" + unit;
        }
      }

      if (legends[0] && legends[0].lastChild) {
        legends[0].lastChild.nodeValue = control.getAttribute("data-legend-a");
      }
      if (legends[1] && legends[1].lastChild) {
        legends[1].lastChild.nodeValue = control.getAttribute("data-legend-b");
      }

      if (numbers[0]) {
        numbers[0].innerHTML = control.getAttribute("data-value-a") + "<em>" + unit + "</em>";
      }
      if (numbers[1]) {
        numbers[1].innerHTML = control.getAttribute("data-value-b") + "<em>" + unit + "</em>";
      }

      bars.forEach(function updateBar(bar, index) {
        bar.style.setProperty("--a", (valuesA[index] || 0) + "%");
        bar.style.setProperty("--b", (valuesB[index] || 0) + "%");
        if (reportMonth > 0) {
          bar.classList.toggle("active", index + 1 === reportMonth);
        }
      });
    }

    controls.forEach(function bindStatsControl(control) {
      control.addEventListener("click", function onStatsControlClick() {
        updateChart(control);
      });
    });

    updateChart(document.querySelector("[data-stats-news].active") || controls[0]);
  }

  function initVideo() {
    var carousel = document.querySelector("[data-video-carousel]");
    if (!carousel) {
      return;
    }

    var pages = getItems(".video-page", carousel);
    var prev = document.querySelector(".video-prev");
    var next = document.querySelector(".video-next");
    var current = 0;

    function showPage(index) {
      current = (index + pages.length) % pages.length;
      setActive(pages, current);
    }

    if (prev) {
      prev.addEventListener("click", function onVideoPrevClick() {
        showPage(current - 1);
      });
    }

    if (next) {
      next.addEventListener("click", function onVideoNextClick() {
        showPage(current + 1);
      });
    }

    showPage(0);
  }

  function initScroll() {
    var navLinks = getItems(".header-nav a");

    navLinks.forEach(function bindNavLink(link) {
      link.addEventListener("mouseenter", function onNavEnter() {
        navLinks.forEach(function updateLink(item) {
          item.classList.toggle("hovering", item === link);
        });
      });
    });

    var nav = document.querySelector(".header-nav");
    if (nav) {
      nav.addEventListener("mouseleave", function onNavLeave() {
        navLinks.forEach(function removeHover(item) {
          item.classList.remove("hovering");
        });
      });
    }
  }

  function initSearch() {
    var form = document.querySelector(".header-search");
    var input = form ? form.querySelector("input") : null;

    if (!form || !input) {
      return;
    }

    form.addEventListener("submit", function onSearchSubmit(event) {
      if (!input.value.trim()) {
        event.preventDefault();
        input.focus();
        return;
      }

      event.preventDefault();
      input.blur();
    });
  }

  function initFooterLinks() {
    getItems("[data-footer-link]").forEach(function bindFooterLink(select) {
      select.addEventListener("change", function onFooterLinkChange() {
        var href = select.value;
        if (!href) {
          return;
        }
        if (/^https?:\/\//i.test(href)) {
          window.open(href, "_blank", "noopener,noreferrer");
        }
        select.selectedIndex = 0;
      });
    });
  }

  function initLinkPolicy() {
    document.addEventListener("click", function validateNavigation(event) {
      var link = event.target.closest ? event.target.closest("a[href]") : null;
      var rawHref;
      var targetURL;

      if (!link) {
        return;
      }
      rawHref = link.getAttribute("href") || "";
      if (!rawHref || rawHref === "#") {
        event.preventDefault();
        return;
      }
      try {
        targetURL = new URL(rawHref, window.location.href);
        if (targetURL.protocol !== "http:" && targetURL.protocol !== "https:" && targetURL.protocol !== "file:") {
          event.preventDefault();
        }
      } catch (error) {
        event.preventDefault();
      }
    });
  }

  function initPage() {
    initLinkPolicy();
    initBanner();
    initTabs();
    initMenu();
    initIndustryMenu();
    initStatsNews();
    initVideo();
    initScroll();
    initSearch();
    initFooterLinks();
  }

  document.addEventListener("DOMContentLoaded", initPage);
})();
