(function initArticleActions() {
  "use strict";

  var shareButton = document.querySelector("[data-share-article]");
  var likeButton = document.querySelector("[data-like-article]");
  var likeLabel = document.querySelector("[data-like-label]");
  var status = document.querySelector("[data-article-action-status]");
  var statusTimer;

  if (!shareButton && !likeButton) return;

  function articleKey() {
    var params = new URLSearchParams(window.location.search);
    var articleID = params.get("id");
    return "caam-article-liked:" + window.location.pathname + (articleID ? "?id=" + articleID : "");
  }

  function showStatus(message) {
    if (!status) return;
    window.clearTimeout(statusTimer);
    status.textContent = message;
    status.classList.add("is-visible");
    statusTimer = window.setTimeout(function hideStatus() {
      status.classList.remove("is-visible");
    }, 1800);
  }

  function setLiked(liked) {
    if (!likeButton) return;
    likeButton.setAttribute("aria-pressed", liked ? "true" : "false");
    if (likeLabel) likeLabel.textContent = liked ? "已点赞" : "点赞";
  }

  if (likeButton) {
    try {
      setLiked(window.localStorage.getItem(articleKey()) === "1");
    } catch (error) {
      setLiked(false);
    }

    likeButton.addEventListener("click", function toggleLike() {
      var liked = likeButton.getAttribute("aria-pressed") !== "true";
      setLiked(liked);
      try {
        if (liked) window.localStorage.setItem(articleKey(), "1");
        else window.localStorage.removeItem(articleKey());
      } catch (error) {
        // The button still works for this page view when browser storage is unavailable.
      }
      showStatus(liked ? "感谢您的点赞" : "已取消点赞");
    });
  }

  function legacyCopy(text) {
    var input = document.createElement("textarea");
    input.value = text;
    input.setAttribute("readonly", "");
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.appendChild(input);
    input.select();
    var copied = document.execCommand("copy");
    document.body.removeChild(input);
    return copied;
  }

  function copyArticleURL() {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(window.location.href);
    }
    return legacyCopy(window.location.href) ? Promise.resolve() : Promise.reject(new Error("copy failed"));
  }

  if (shareButton) {
    shareButton.addEventListener("click", function shareArticle() {
      var shareData = {
        title: document.title,
        text: document.querySelector(".article-header h1") ? document.querySelector(".article-header h1").textContent : document.title,
        url: window.location.href
      };

      if (navigator.share) {
        navigator.share(shareData).catch(function handleShareError(error) {
          if (error && error.name === "AbortError") return;
          copyArticleURL().then(function copied() {
            showStatus("链接已复制");
          }).catch(function copyFailed() {
            showStatus("分享失败，请复制浏览器地址");
          });
        });
        return;
      }

      copyArticleURL().then(function copied() {
        showStatus("链接已复制");
      }).catch(function copyFailed() {
        showStatus("分享失败，请复制浏览器地址");
      });
    });
  }
})();
