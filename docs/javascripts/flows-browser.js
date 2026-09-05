(function () {
  "use strict";

  // The generated catalog holds >1k call-graph flows; cap DOM rows per render.
  var RENDER_LIMIT = 60;

  function catalogURL() {
    var path =
      window.__FLOW_CATALOG_PATH__ ||
      window.__FLOW_CATALOG_URL__ ||
      "flows/flows.json";
    if (/^https?:\/\//.test(path)) {
      return path;
    }
    var base =
      typeof __md_scope !== "undefined"
        ? __md_scope
        : new URL("/", document.baseURI);
    if (path.charAt(0) === "/") {
      return new URL(path, base).href;
    }
    return new URL(path, base).href;
  }

  function boot() {
    var root = document.getElementById("flow-browser");
    var elSearch = document.getElementById("flow-search");
    var elResults = document.getElementById("flow-results");
    var elDetail = document.getElementById("flow-detail");
    var elFilters = document.getElementById("flow-filters");

    if (!root || !elSearch || !elResults || !elDetail) {
      return;
    }
    if (root.dataset.flowReady === "1") {
      return;
    }
    root.dataset.flowReady = "1";

    /** @type {{ repo: string, branch: string, flows: Flow[] }} */
    var catalog = { repo: "yairgd/gdbforge", branch: "main", flows: [] };
    /** @type {Fuse<Flow>} */
    var fuse = null;
    var selectedID = "";
    var backendFilter = new Set(["gdb", "dlv", "both"]);
    var hideAuto = false;

    init();

    async function init() {
      elResults.innerHTML =
        '<p class="flow-browser__empty">Loading flows…</p>';
      elDetail.hidden = true;

      try {
        var url = catalogURL();
        var resp = await fetch(url);
        if (!resp.ok) {
          throw new Error("fetch " + url + ": HTTP " + resp.status);
        }
        catalog = await resp.json();
        if (!catalog.flows || !catalog.flows.length) {
          throw new Error("flows.json has no flows[] entries");
        }
      } catch (err) {
        elResults.innerHTML =
          '<p class="flow-browser__empty">Failed to load flows.json (' +
          escapeHTML(catalogURL()) +
          "): " +
          escapeHTML(String(err)) +
          ". Use the catalog table above or open flows.json directly.</p>";
        return;
      }

      buildFuse();
      buildFilters();
      elSearch.addEventListener("input", renderResults);
      window.addEventListener("hashchange", onHashChange);
      renderResults();
    }

    function buildFuse() {
      if (typeof Fuse === "undefined") {
        elResults.innerHTML =
          '<p class="flow-browser__empty">Fuse.js failed to load.</p>';
        return;
      }
      var docs = catalog.flows.map(enrichFlow);
      fuse = new Fuse(docs, {
        keys: [
          { name: "title", weight: 0.35 },
          { name: "keywords", weight: 0.3 },
          { name: "trigger", weight: 0.15 },
          { name: "symbolText", weight: 0.2 },
        ],
        threshold: 0.38,
        ignoreLocation: true,
      });
    }

    function enrichFlow(flow) {
      var symbols = (flow.steps || []).map(function (s) {
        return s.symbol || "";
      });
      return Object.assign({}, flow, {
        symbolText: symbols.join(" "),
      });
    }

    function isAutoFlow(flow) {
      return flow.auto === true || (flow.id && flow.id.indexOf("auto-") === 0);
    }

    function countSummary(shown, total) {
      var curated = 0;
      var auto = 0;
      catalog.flows.forEach(function (f) {
        if (isAutoFlow(f)) {
          auto++;
        } else {
          curated++;
        }
      });
      if (shown === total) {
        return total + " flows (" + curated + " curated · " + auto + " auto)";
      }
      return shown + " of " + total + " flows";
    }

    function buildFilters() {
      if (!elFilters) {
        return;
      }
      var backends = [
        { id: "gdb", label: "GDB" },
        { id: "dlv", label: "Delve" },
        { id: "both", label: "Both / cmdline" },
      ];
      elFilters.innerHTML = "";
      backends.forEach(function (b) {
        var label = document.createElement("label");
        var cb = document.createElement("input");
        cb.type = "checkbox";
        cb.checked = true;
        cb.dataset.backend = b.id;
        cb.addEventListener("change", function () {
          if (cb.checked) {
            backendFilter.add(b.id);
          } else {
            backendFilter.delete(b.id);
          }
          renderResults();
        });
        label.appendChild(cb);
        label.appendChild(document.createTextNode(b.label));
        elFilters.appendChild(label);
      });

      var curatedLabel = document.createElement("label");
      var curatedCb = document.createElement("input");
      curatedCb.type = "checkbox";
      curatedCb.checked = false;
      curatedCb.addEventListener("change", function () {
        hideAuto = curatedCb.checked;
        renderResults();
      });
      curatedLabel.appendChild(curatedCb);
      curatedLabel.appendChild(document.createTextNode("Curated only"));
      elFilters.appendChild(curatedLabel);
    }

    // parkDetail returns the detail panel to the container so a list rebuild
    // cannot delete it; placeDetail re-anchors it under the selected row.
    function parkDetail() {
      if (elDetail.parentNode !== root) {
        root.appendChild(elDetail);
      }
    }

    function placeDetail(id) {
      var rows = elResults.querySelectorAll(".flow-browser__result");
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].dataset.flowId === id) {
          rows[i].insertAdjacentElement("afterend", elDetail);
          return true;
        }
      }
      parkDetail();
      return false;
    }

    function matchesBackend(flow) {
      var tags = flow.backend || [];
      if (tags.length === 0) {
        return true;
      }
      for (var i = 0; i < tags.length; i++) {
        if (backendFilter.has(tags[i])) {
          return true;
        }
      }
      return false;
    }

    function renderResults() {
      if (!fuse) {
        return;
      }
      var q = elSearch.value.trim();
      var flows = catalog.flows.slice();
      if (q) {
        flows = fuse.search(q).map(function (r) {
          return r.item;
        });
      }
      flows = flows.filter(matchesBackend);
      if (hideAuto) {
        flows = flows.filter(function (f) {
          return !isAutoFlow(f);
        });
      }

      // The detail panel is parked inside the list (under its row), so pull it
      // out before wiping the list or it would be destroyed with the rows.
      parkDetail();

      elResults.innerHTML = "";
      if (flows.length === 0) {
        elResults.innerHTML =
          '<p class="flow-browser__empty">No matching flows (try clearing filters or search).</p>';
        elDetail.hidden = true;
        return;
      }

      var total = flows.length;
      if (total > RENDER_LIMIT) {
        flows = flows.slice(0, RENDER_LIMIT);
      }

      // The open flow must have a row on screen, otherwise its tree would have
      // nowhere to anchor (happens for deep links past the render cap).
      var pinnedID = location.hash.replace(/^#/, "") || selectedID;
      if (
        pinnedID &&
        !flows.some(function (f) {
          return f.id === pinnedID;
        })
      ) {
        var pinned = catalog.flows.find(function (f) {
          return f.id === pinnedID;
        });
        if (pinned) {
          flows.unshift(pinned);
        }
      }

      var heading = document.createElement("p");
      heading.className = "flow-browser__count";
      heading.textContent = countSummary(total, catalog.flows.length);
      if (total > RENDER_LIMIT) {
        heading.textContent +=
          " — showing first " + RENDER_LIMIT + ", refine the search to narrow";
      }
      elResults.appendChild(heading);

      flows.forEach(function (flow) {
        var btn = document.createElement("button");
        btn.type = "button";
        btn.className = "flow-browser__result";
        btn.dataset.flowId = flow.id;
        if (flow.id === selectedID) {
          btn.classList.add("is-selected");
        }
        btn.setAttribute("aria-expanded", flow.id === selectedID ? "true" : "false");
        btn.setAttribute("aria-controls", "flow-detail");
        btn.innerHTML =
          (isAutoFlow(flow)
            ? '<span class="flow-browser__badge">auto</span> '
            : "") +
          '<div class="flow-browser__result-title">' +
          escapeHTML(flow.title) +
          "</div>" +
          '<div class="flow-browser__result-meta">' +
          escapeHTML((flow.backend || []).join(", ")) +
          " · " +
          escapeHTML(truncate(flow.trigger || "", 72)) +
          "</div>";
        btn.addEventListener("click", function () {
          if (selectedID === flow.id && !elDetail.hidden) {
            collapseDetail();
            return;
          }
          selectFlow(flow.id, true);
        });
        elResults.appendChild(btn);
      });

      if (selectedID) {
        placeDetail(selectedID);
      }

      var hashID = location.hash.replace(/^#/, "");
      if (hashID) {
        var match = catalog.flows.find(function (f) {
          return f.id === hashID;
        });
        if (match) {
          selectFlow(hashID, false);
        }
      } else if (!selectedID && flows.length === 1 && q) {
        selectFlow(flows[0].id, true);
      }
    }

    function onHashChange() {
      renderResults();
    }

    function selectFlow(id, pushHash) {
      var changed = selectedID !== id;
      selectedID = id;
      var flow = catalog.flows.find(function (f) {
        return f.id === id;
      });
      if (!flow) {
        elDetail.hidden = true;
        return;
      }
      if (pushHash) {
        history.replaceState(null, "", "#" + id);
      }

      elResults.querySelectorAll(".flow-browser__result").forEach(function (btn) {
        var isSel = btn.dataset.flowId === id;
        btn.classList.toggle("is-selected", isSel);
        btn.setAttribute("aria-expanded", isSel ? "true" : "false");
      });

      elDetail.hidden = false;
      elDetail.innerHTML =
        "<h3>" +
        escapeHTML(flow.title) +
        "</h3>" +
        '<p class="flow-browser__trigger"><strong>Trigger:</strong> ' +
        escapeHTML(flow.trigger || "") +
        "</p>" +
        '<pre class="flow-browser__tree">' +
        renderTree(flow) +
        "</pre>" +
        renderBreakpoints(flow);

      // Show the tree directly under its row rather than at the page bottom.
      var anchored = placeDetail(id);
      if (changed && (!anchored || !pushHash)) {
        elDetail.scrollIntoView({ block: "nearest" });
      }
    }

    function collapseDetail() {
      selectedID = "";
      elDetail.hidden = true;
      elDetail.innerHTML = "";
      parkDetail();
      elResults.querySelectorAll(".flow-browser__result").forEach(function (btn) {
        btn.classList.remove("is-selected");
        btn.setAttribute("aria-expanded", "false");
      });
      if (location.hash.length > 1) {
        history.replaceState(null, "", location.pathname + location.search);
      }
    }

    function renderTree(flow) {
      var steps = flow.steps || [];
      var lines = [];
      for (var i = 0; i < steps.length; i++) {
        var step = steps[i];
        var indent = step.indent || 0;
        var prefix = treePrefix(indent, i, steps);
        var sym = escapeHTML(step.symbol || "?");
        var loc = stepLink(step);
        var note = step.note
          ? "  // " + escapeHTML(step.note)
          : step.branch
            ? "  [" + escapeHTML(step.branch) + "]"
            : "";
        lines.push(prefix + sym + (loc ? "  " + loc : "") + note);
      }
      return lines.join("\n");
    }

    function treePrefix(indent, index, steps) {
      if (indent <= 0) {
        return "";
      }
      var s = "  ";
      for (var d = 1; d < indent; d++) {
        s += "   ";
      }
      var next = steps[index + 1];
      var nextIndent = next ? next.indent || 0 : 0;
      if (nextIndent > indent) {
        return s + "└─ ";
      }
      return s + "├─ ";
    }

    function stepLink(step) {
      if (!step.file) {
        return "";
      }
      var line = step.line || 1;
      var path = step.file;
      var label = path + ":" + line;
      var url = githubURL(path, line);
      return (
        '<a href="' +
        escapeAttr(url) +
        '" target="_blank" rel="noopener">' +
        escapeHTML(label) +
        "</a>"
      );
    }

    function githubURL(file, line) {
      var repo = catalog.repo || "yairgd/gdbforge";
      var branch = catalog.branch || "main";
      return (
        "https://github.com/" +
        repo +
        "/blob/" +
        branch +
        "/" +
        file +
        "#L" +
        line
      );
    }

    function renderBreakpoints(flow) {
      var bps = flow.breakpoints || [];
      if (bps.length === 0) {
        return "";
      }
      var html =
        '<div class="flow-browser__bps"><strong>Delve/GDB breakpoints:</strong>';
      for (var i = 0; i < bps.length; i++) {
        html += "<code>" + escapeHTML(bps[i]) + "</code>";
      }
      html += "</div>";
      return html;
    }

    function truncate(s, n) {
      if (s.length <= n) {
        return s;
      }
      return s.slice(0, n - 1) + "…";
    }

    function escapeHTML(s) {
      return String(s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    }

    function escapeAttr(s) {
      return escapeHTML(s).replace(/'/g, "&#39;");
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }

  if (typeof document$ !== "undefined") {
    document$.subscribe(function () {
      if (document.getElementById("flow-browser")) {
        boot();
      }
    });
  }
})();
