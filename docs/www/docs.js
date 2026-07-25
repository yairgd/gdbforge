/* gdbforge docs viewer — Markdown + Mermaid (CDN, classic scripts). */

const MERMAID_CDN = "https://cdn.jsdelivr.net/npm/mermaid@10.9.3/dist/mermaid.min.js";
const MARKED_CDN = "https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js";

function getBase() {
  const meta = document.querySelector('meta[name="docs-base"]');
  if (!meta || !meta.content || meta.content === "/") {
    return "/";
  }
  let base = meta.content;
  if (!base.startsWith("/")) {
    base = "/" + base;
  }
  if (!base.endsWith("/")) {
    base += "/";
  }
  return base;
}

function siteUrl(path) {
  const pathPart = path.startsWith("/") ? path.slice(1) : path;
  const base = getBase();
  if (base === "/") {
    return "/" + pathPart;
  }
  return base + pathPart;
}

function formatError(err) {
  if (err instanceof Error) {
    return err.message + (err.stack ? `\n${err.stack}` : "");
  }
  if (typeof err === "string") {
    return err;
  }
  if (err && typeof err.message === "string") {
    return err.message;
  }
  if (err && typeof err.str === "string") {
    return err.str;
  }
  try {
    return JSON.stringify(err, null, 2);
  } catch {
    return String(err);
  }
}

async function loadScript(src) {
  if (document.querySelector(`script[src="${src}"]`)) {
    return;
  }
  await new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = src;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error(`Failed to load script: ${src}`));
    document.head.appendChild(s);
  });
}

async function initLibs() {
  await loadScript(MARKED_CDN);
  await loadScript(MERMAID_CDN);
  if (typeof marked === "undefined") {
    throw new Error("marked library did not load");
  }
  if (typeof mermaid === "undefined") {
    throw new Error("mermaid library did not load");
  }
  marked.setOptions({ gfm: true, breaks: true });
  mermaid.initialize({
    startOnLoad: false,
    theme: "dark",
    securityLevel: "loose",
  });
}

async function fetchText(url) {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`${url}: HTTP ${res.status} ${res.statusText}`);
  }
  return res.text();
}

function extractMermaidBlocks(md) {
  const re = /```mermaid\r?\n([\s\S]*?)```/g;
  const blocks = [];
  let match;
  let i = 0;
  while ((match = re.exec(md)) !== null) {
    blocks.push({ id: `mermaid-embed-${i++}`, source: match[1].trim() });
  }
  return blocks;
}

function replaceMermaidWithPlaceholders(md) {
  let i = 0;
  return md.replace(/```mermaid\r?\n[\s\S]*?```/g, () => {
    return `<div class="mermaid-wrap"><div class="mermaid" id="mermaid-embed-${i++}"></div></div>`;
  });
}

function sanitizeMermaidSource(src) {
  return src.replace(/<br\s*\/?>/gi, "\n");
}

async function renderOneMermaid(el) {
  const source = sanitizeMermaidSource(el.textContent);
  const id = `mermaid-svg-${Math.random().toString(36).slice(2)}`;
  const { svg } = await mermaid.render(id, source);
  el.innerHTML = svg;
}

async function renderMermaidInPage() {
  const nodes = document.querySelectorAll(".markdown-body .mermaid");
  for (const el of nodes) {
    try {
      await renderOneMermaid(el);
    } catch (err) {
      el.innerHTML = `<pre class="error">${escapeHtml(formatError(err))}</pre>`;
      console.error("Mermaid block failed:", err);
    }
  }
}

function diagramPagePath(mermaidName) {
  const name = String(mermaidName || "");
  if (name.endsWith(".html")) {
    return `diagrams/${encodeURIComponent(name)}`;
  }
  const base = name.endsWith(".mermaid") ? name.slice(0, -".mermaid".length) : name;
  return `diagrams/${encodeURIComponent(base + ".html")}`;
}

function fixInternalDocLinks(html) {
  const base = getBase();
  const basePrefix = base === "/" ? "/" : base;

  return html.replace(/href="([^"#?]+)(#[^"]*)?"/g, (match, href, hash) => {
    const fragment = hash || "";
    if (href.startsWith("http://") || href.startsWith("https://") || href.startsWith("mailto:")) {
      return match;
    }
    if (href.startsWith(basePrefix)) {
      return match;
    }
    if (href.endsWith(".md")) {
      const name = href.split("/").pop();
      if (name === "README.md") {
        return `href="${siteUrl("")}${fragment}"`;
      }
      return `href="${siteUrl(`doc/${encodeURIComponent(name)}`)}${fragment}"`;
    }
    if (href.includes("diagrams/") && href.endsWith(".mermaid")) {
      const name = href.split("/").pop();
      return `href="${siteUrl(diagramPagePath(name))}${fragment}"`;
    }
    return match;
  });
}

async function renderMarkdownPage(container, mdFile) {
  const md = await fetchText(siteUrl(`raw/${encodeURIComponent(mdFile)}`));
  window.__mermaidBlocks = extractMermaidBlocks(md);
  const html = fixInternalDocLinks(marked.parse(replaceMermaidWithPlaceholders(md)));
  container.innerHTML = `<article class="markdown-body">${html}</article>`;

  for (const block of window.__mermaidBlocks) {
    const el = document.getElementById(block.id);
    if (el) {
      el.textContent = block.source;
    }
  }

  await renderMermaidInPage();
}

async function renderReadme(container) {
  await renderMarkdownPage(container, "README.md");
}

async function renderDiagram(container, name) {
  const safe = name.replace(/[^a-zA-Z0-9_.-]/g, "");
  const src = await fetchText(siteUrl(`raw/diagrams/${encodeURIComponent(safe)}`));
  container.innerHTML = `
    <h2>${escapeHtml(safe)}</h2>
    <p><a href="${siteUrl("")}">← Documentation index</a> · <a href="${siteUrl("diagrams/")}">All diagrams</a></p>
    <div class="mermaid-wrap mermaid-standalone">
      <div class="mermaid"></div>
    </div>
    <details>
      <summary>Source (.mermaid)</summary>
      <pre><code>${escapeHtml(src)}</code></pre>
    </details>`;
  const node = container.querySelector(".mermaid");
  node.textContent = src;
  try {
    await renderOneMermaid(node);
  } catch (err) {
    node.innerHTML = `<pre class="error">${escapeHtml(formatError(err))}</pre>`;
    throw err;
  }
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function parseDiagramList() {
  const b64 = document.body.dataset.diagramsB64;
  if (b64) {
    return JSON.parse(atob(b64));
  }
  const raw = document.body.dataset.diagrams;
  if (raw) {
    return JSON.parse(raw);
  }
  return [];
}

async function renderDiagramIndex(container, names) {
  const items = names
    .map((n) => `<li><a href="${siteUrl(diagramPagePath(n))}">${escapeHtml(n)}</a></li>`)
    .join("");
  container.innerHTML = `
    <h2>Mermaid diagrams</h2>
    <p>Standalone diagram pages (sources in <code>docs/diagrams/</code>).</p>
    <ul class="diagram-list">${items}</ul>`;
}

async function main() {
  const root = document.getElementById("content");
  const page = document.body.dataset.page;

  try {
    await initLibs();
    if (page === "readme") {
      await renderReadme(root);
    } else if (page === "doc") {
      await renderMarkdownPage(root, document.body.dataset.md);
    } else if (page === "diagram") {
      await renderDiagram(root, document.body.dataset.diagram);
    } else if (page === "diagrams") {
      await renderDiagramIndex(root, parseDiagramList());
    } else {
      throw new Error(`Unknown page type: ${page}`);
    }
  } catch (err) {
    root.innerHTML = `<div class="error"><strong>Error</strong><pre>${escapeHtml(formatError(err))}</pre></div>`;
    console.error(err);
  }
}

document.addEventListener("DOMContentLoaded", main);
