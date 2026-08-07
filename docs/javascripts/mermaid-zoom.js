(function () {
  // Working UI: lightbox + Zoom in/out/Reset/Close + local Panzoom.
  //
  // Playwright findings:
  // 1) Capture pre>code textContent synchronously at script load (sampleLen=1142).
  // 2) mermaid.run() ate data-pz-* attrs and produced error SVGs (code-wrapper path).
  // 3) Page + lightbox both use mermaid.render(storedSource) instead.

  console.info("[mermaid-zoom] pz-box-10");

  var session = null;
  var sourcesByIndex = [];

  (function captureNow() {
    var nodes = document.querySelectorAll("pre.mermaid, .mermaid");
    sourcesByIndex = [];
    nodes.forEach(function (el, index) {
      var code = el.querySelector("code");
      var src = code
        ? String(code.textContent || "").replace(/\u00a0/g, " ").trim()
        : "";
      sourcesByIndex[index] = src;
      el.setAttribute("data-pz-i", String(index));
      if (src) {
        try {
          el.setAttribute("data-pz-src", encodeURIComponent(src));
        } catch (e) {
          /* index array still holds source */
        }
      }
    });
    console.info(
      "[mermaid-zoom] early capture diagrams=",
      nodes.length,
      "withSrc=",
      sourcesByIndex.filter(Boolean).length,
      "sampleLen=",
      sourcesByIndex[0] ? sourcesByIndex[0].length : 0
    );
  })();

  if (typeof mermaid !== "undefined") {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: "dark",
    });
  }

  function theme() {
    var scheme =
      document.body && document.body.getAttribute("data-md-color-scheme");
    if (scheme === "slate") {
      return "dark";
    }
    if (scheme === "default") {
      return "default";
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "default";
  }

  function bindSourceAttrs(el, index) {
    el.setAttribute("data-pz-i", String(index));
    var src = sourcesByIndex[index] || "";
    if (src) {
      try {
        el.setAttribute("data-pz-src", encodeURIComponent(src));
      } catch (e) {
        /* ignore */
      }
    }
  }

  function readSource(el) {
    var attr = el.getAttribute("data-pz-src");
    if (attr) {
      try {
        return decodeURIComponent(attr);
      } catch (e) {
        /* ignore */
      }
    }
    var idx = el.getAttribute("data-pz-i");
    if (idx !== null && sourcesByIndex[Number(idx)]) {
      return sourcesByIndex[Number(idx)];
    }
    return "";
  }

  function svgLooksLikeError(svg) {
    if (!svg) {
      return true;
    }
    return (
      svg.indexOf("Syntax error") !== -1 &&
      svg.indexOf("mermaid version") !== -1
    );
  }

  function cleanupRenderId(renderId) {
    var node = document.getElementById(renderId);
    if (node && node.parentNode) {
      node.parentNode.removeChild(node);
    }
  }

  function ensureShell() {
    var root = document.getElementById("pz-lightbox");
    if (root) {
      return root;
    }

    root = document.createElement("div");
    root.id = "pz-lightbox";
    root.hidden = true;
    root.innerHTML =
      '<div class="pz-lightbox__backdrop" data-pz-close></div>' +
      '<div class="pz-lightbox__panel">' +
      '  <div class="pz-lightbox__controls">' +
      '    <button type="button" id="pz-zoom-in">Zoom in</button>' +
      '    <button type="button" id="pz-zoom-out">Zoom out</button>' +
      '    <button type="button" id="pz-reset">Reset</button>' +
      '    <button type="button" id="pz-close" data-pz-close>Close</button>' +
      "  </div>" +
      '  <div class="panzoom-parent" id="pz-parent"></div>' +
      "</div>";
    document.body.appendChild(root);

    root.addEventListener("click", function (event) {
      if (event.target.closest("[data-pz-close]")) {
        closeViewer();
      }
    });

    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape" && !root.hidden) {
        closeViewer();
      }
    });

    return root;
  }

  function closeViewer() {
    if (session) {
      var s = session;
      session = null;
      if (s.parent && s.zoomWithWheel) {
        s.parent.removeEventListener("wheel", s.zoomWithWheel);
      }
      try {
        if (s.panzoom) {
          s.panzoom.destroy();
        }
      } catch (e) {
        /* ignore */
      }
      if (s.blobUrl) {
        URL.revokeObjectURL(s.blobUrl);
      }
      s.parent.innerHTML = "";
      s.root.hidden = true;
    } else {
      var root = document.getElementById("pz-lightbox");
      if (root) {
        root.hidden = true;
      }
    }
    document.body.classList.remove("pz-lightbox-open");
  }

  function sizeFromSvgXml(xml) {
    var w = Math.max(480, Math.floor(window.innerWidth * 0.85));
    var h = Math.max(320, Math.floor(window.innerHeight * 0.75));
    var wm = xml.match(/\bwidth="([\d.]+)"/);
    var hm = xml.match(/\bheight="([\d.]+)"/);
    var vbm = xml.match(/\bviewBox="([^"]+)"/);
    if (wm && hm) {
      var pw = parseFloat(wm[1]);
      var ph = parseFloat(hm[1]);
      if (pw > 40 && ph > 40) {
        w = Math.round(pw);
        h = Math.round(ph);
      }
    } else if (vbm) {
      var parts = vbm[1].split(/[\s,]+/).map(parseFloat);
      if (parts.length === 4 && parts[2] > 40 && parts[3] > 40) {
        w = Math.round(parts[2]);
        h = Math.round(parts[3]);
      }
    }
    var maxW = Math.floor(window.innerWidth * 0.9);
    var maxH = Math.floor(window.innerHeight * 0.8);
    var scale = Math.min(maxW / w, maxH / h, 2.5);
    if (scale > 1.05) {
      w = Math.round(w * scale);
      h = Math.round(h * scale);
    }
    return { w: w, h: h };
  }

  function showSvgXml(xml) {
    if (!xml || svgLooksLikeError(xml)) {
      return false;
    }
    if (xml.indexOf("xmlns=") === -1) {
      xml = xml.replace("<svg", '<svg xmlns="http://www.w3.org/2000/svg"');
    }
    var size = sizeFromSvgXml(xml);
    xml = xml.replace(/<svg\b([^>]*)>/, function (full, attrs) {
      var a = attrs
        .replace(/\swidth="[^"]*"/g, "")
        .replace(/\sheight="[^"]*"/g, "");
      return '<svg' + a + ' width="' + size.w + '" height="' + size.h + '">';
    });

    var root = ensureShell();
    var parent = root.querySelector("#pz-parent");
    var zoomIn = root.querySelector("#pz-zoom-in");
    var zoomOut = root.querySelector("#pz-zoom-out");
    var reset = root.querySelector("#pz-reset");

    var blobUrl = URL.createObjectURL(
      new Blob([xml], { type: "image/svg+xml;charset=utf-8" })
    );
    var img = document.createElement("img");
    img.alt = "Diagram";
    img.draggable = false;
    img.width = size.w;
    img.height = size.h;
    img.style.width = size.w + "px";
    img.style.height = size.h + "px";
    img.style.maxWidth = "none";
    img.style.maxHeight = "none";
    img.style.display = "block";
    img.src = blobUrl;

    parent.innerHTML = "";
    parent.appendChild(img);
    root.hidden = false;
    document.body.classList.add("pz-lightbox-open");

    function attach() {
      var panzoom = Panzoom(img, {
        cursor: "grab",
        maxScale: 5,
        minScale: 0.2,
      });
      parent.addEventListener("wheel", panzoom.zoomWithWheel);
      zoomIn.onclick = panzoom.zoomIn;
      zoomOut.onclick = panzoom.zoomOut;
      reset.onclick = panzoom.reset;
      session = {
        root: root,
        parent: parent,
        panzoom: panzoom,
        zoomWithWheel: panzoom.zoomWithWheel,
        blobUrl: blobUrl,
      };
      console.info("[mermaid-zoom] lightbox ready scale=", panzoom.getScale());
    }

    if (img.complete) {
      attach();
    } else {
      img.onload = attach;
      img.onerror = function () {
        URL.revokeObjectURL(blobUrl);
        parent.innerHTML =
          '<p style="color:#e6edf3;padding:2rem">Could not display diagram image.</p>';
        session = { root: root, parent: parent };
      };
    }
    return true;
  }

  function openViewer(diagram) {
    if (typeof Panzoom !== "function") {
      alert(
        "Panzoom failed to load. Check that javascripts/vendor/panzoom.min.js is present."
      );
      return;
    }

    closeViewer();
    var root = ensureShell();
    var parent = root.querySelector("#pz-parent");
    parent.innerHTML =
      '<p style="color:#e6edf3;padding:2rem;font:16px/1.4 system-ui,sans-serif">Loading diagram…</p>';
    root.hidden = false;
    document.body.classList.add("pz-lightbox-open");
    session = { root: root, parent: parent };

    var source = readSource(diagram);
    if (!source) {
      parent.innerHTML =
        '<p style="color:#e6edf3;padding:2rem">Diagram source missing.</p>';
      return;
    }

    var renderId =
      "pz-lb-" + Date.now() + "-" + Math.random().toString(36).slice(2, 8);

    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: theme(),
    });

    Promise.resolve(mermaid.render(renderId, source))
      .then(function (out) {
        cleanupRenderId(renderId);
        if (!out || svgLooksLikeError(out.svg)) {
          parent.innerHTML =
            '<p style="color:#e6edf3;padding:2rem">Could not render diagram.</p>';
          return;
        }
        showSvgXml(out.svg);
      })
      .catch(function (err) {
        cleanupRenderId(renderId);
        console.error("[mermaid-zoom] lightbox render failed", err);
        parent.innerHTML =
          '<p style="color:#e6edf3;padding:2rem">Could not render diagram.</p>';
      });
  }

  function markZoomable(el) {
    el.classList.remove("mermaid");
    el.classList.add("mermaid-diagram", "mermaid--zoomable");
    el.title = "Click to enlarge";
    el.setAttribute("role", "button");
    el.tabIndex = 0;
  }

  document.addEventListener("click", function (event) {
    if (event.target.closest("#pz-lightbox")) {
      return;
    }
    var diagram = event.target.closest(
      ".mermaid-diagram, pre.mermaid, .mermaid"
    );
    if (!diagram) {
      return;
    }
    event.preventDefault();
    openViewer(diagram);
  });

  function boot() {
    if (typeof mermaid === "undefined") {
      console.error("[mermaid-zoom] mermaid missing");
      return;
    }

    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: theme(),
    });

    var nodes = Array.prototype.slice.call(
      document.querySelectorAll("pre.mermaid, .mermaid")
    );

    // Ensure every node still has an index even if Mermaid replaced siblings.
    nodes.forEach(function (el, index) {
      if (!el.getAttribute("data-pz-i")) {
        bindSourceAttrs(el, index);
      }
    });

    var chain = Promise.resolve();
    nodes.forEach(function (el, index) {
      chain = chain.then(function () {
        var source =
          readSource(el) ||
          sourcesByIndex[index] ||
          "";
        bindSourceAttrs(el, index);
        if (!source) {
          markZoomable(el);
          return;
        }
        // Skip if a good SVG is already present.
        var existing = el.querySelector("svg");
        if (
          existing &&
          existing.getAttribute("aria-roledescription") !== "error" &&
          !svgLooksLikeError(existing.textContent || "")
        ) {
          markZoomable(el);
          return;
        }

        var renderId = "pz-page-" + index + "-" + Date.now();
        return Promise.resolve(mermaid.render(renderId, source))
          .then(function (out) {
            cleanupRenderId(renderId);
            if (out && out.svg && !svgLooksLikeError(out.svg)) {
              el.innerHTML = out.svg;
            }
            bindSourceAttrs(el, index);
            markZoomable(el);
          })
          .catch(function (err) {
            cleanupRenderId(renderId);
            console.error("[mermaid-zoom] page render failed", index, err);
            bindSourceAttrs(el, index);
            markZoomable(el);
          });
      });
    });

    chain
      .then(function () {
        ensureShell();
        console.info(
          "[mermaid-zoom] ready withSrc=",
          document.querySelectorAll("[data-pz-src]").length,
          "okSvg=",
          Array.prototype.slice
            .call(document.querySelectorAll(".mermaid-diagram svg"))
            .filter(function (s) {
              return s.getAttribute("aria-roledescription") !== "error";
            }).length
        );
      })
      .catch(function (err) {
        console.error(err);
        ensureShell();
      });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    setTimeout(boot, 0);
  }
})();
