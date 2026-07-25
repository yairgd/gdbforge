/* Render Jekyll ```mermaid fenced blocks (pre > code.language-mermaid). */
(function () {
  var MERMAID_CDN =
    "https://cdn.jsdelivr.net/npm/mermaid@10.9.3/dist/mermaid.min.js";

  function convertBlocks() {
    document.querySelectorAll("pre code.language-mermaid").forEach(function (code) {
      var pre = code.parentElement;
      if (!pre || pre.tagName !== "PRE") {
        return;
      }
      var div = document.createElement("div");
      div.className = "mermaid";
      div.textContent = code.textContent;
      pre.replaceWith(div);
    });
  }

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      if (document.querySelector('script[src="' + src + '"]')) {
        resolve();
        return;
      }
      var s = document.createElement("script");
      s.src = src;
      s.onload = function () {
        resolve();
      };
      s.onerror = function () {
        reject(new Error("Failed to load " + src));
      };
      document.head.appendChild(s);
    });
  }

  function boot() {
    convertBlocks();
    if (typeof mermaid === "undefined") {
      return;
    }
    mermaid.initialize({
      startOnLoad: false,
      theme: "dark",
      securityLevel: "loose",
    });
    return mermaid.run({ querySelector: ".mermaid" });
  }

  function start() {
    loadScript(MERMAID_CDN)
      .then(boot)
      .catch(function (err) {
        console.error("jekyll-mermaid:", err);
      });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
