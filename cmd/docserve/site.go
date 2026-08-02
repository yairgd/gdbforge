package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func normalizeBase(base string) string {
	base = strings.TrimSpace(base)
	if base == "" || base == "/" {
		return "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

func joinBase(base, path string) string {
	base = normalizeBase(base)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if base == "/" {
		return path
	}
	return strings.TrimSuffix(base, "/") + path
}

// diagramPageFile maps a source name (e.g. data_flow.mermaid) to the static HTML
// filename GitHub Pages can serve as text/html (unknown .mermaid triggers downloads).
func diagramPageFile(mermaidName string) string {
	base := strings.TrimSuffix(mermaidName, ".mermaid")
	if base == mermaidName || base == "" {
		return mermaidName + ".html"
	}
	return base + ".html"
}

// diagramSourceName maps a diagrams URL leaf back to the .mermaid source file.
func diagramSourceName(pageLeaf string) string {
	pageLeaf = strings.TrimSuffix(pageLeaf, "/")
	switch {
	case strings.HasSuffix(pageLeaf, ".html"):
		base := strings.TrimSuffix(pageLeaf, ".html")
		if strings.HasSuffix(base, ".mermaid") {
			return base
		}
		return base + ".mermaid"
	case strings.HasSuffix(pageLeaf, ".mermaid"):
		return pageLeaf
	default:
		return pageLeaf + ".mermaid"
	}
}

// docPageFile maps a Markdown source (e.g. OVERVIEW.md) to a static HTML filename.
// Publishing HTML as *.md makes GitHub Pages serve text/markdown (raw text in the browser).
func docPageFile(mdName string) string {
	base := strings.TrimSuffix(mdName, ".md")
	if base == mdName || base == "" {
		return mdName + ".html"
	}
	return base + ".html"
}

// docSourceName maps a /doc/ URL leaf back to the .md source file.
func docSourceName(pageLeaf string) string {
	pageLeaf = strings.TrimSuffix(pageLeaf, "/")
	switch {
	case strings.HasSuffix(pageLeaf, ".html"):
		base := strings.TrimSuffix(pageLeaf, ".html")
		if strings.HasSuffix(base, ".md") {
			return base
		}
		return base + ".md"
	case strings.HasSuffix(pageLeaf, ".md"):
		return pageLeaf
	default:
		return pageLeaf + ".md"
	}
}

func (s *docServer) navLinks(base string) string {
	parts := []string{fmt.Sprintf(`<a href="%s">Index</a>`, html.EscapeString(joinBase(base, "/")))}
	for _, name := range s.listDocPages() {
		if name == "README.md" {
			continue
		}
		label := strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "_", " ")
		href := joinBase(base, "/doc/"+url.PathEscape(docPageFile(name)))
		parts = append(parts, fmt.Sprintf(`<a href="%s">%s</a>`, href, html.EscapeString(label)))
	}
	parts = append(parts, fmt.Sprintf(`<a href="%s">Diagrams</a>`, html.EscapeString(joinBase(base, "/diagrams/"))))
	return strings.Join(parts, "\n      ")
}

func pageShell(seo pageSEO, bodyAttrs, nav, placeholder string) []byte {
	if placeholder == "" {
		placeholder = "<p>Loading…</p>"
	}
	base := normalizeBase(seo.Base)
	metaBase := html.EscapeString(base)
	cssHref := html.EscapeString(joinBase(base, "/www/site.css"))
	jsHref := html.EscapeString(joinBase(base, "/www/docs.js"))
	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="docs-base" content="%s">
%s  <link rel="stylesheet" href="%s">
</head>
<body %s>
  <header>
    <h1>gdbforge Documentation</h1>
    <nav>
      %s
    </nav>
  </header>
  <main id="content">%s</main>
  <footer>gdbforge docs · Markdown + Mermaid via CDN</footer>
  <script src="%s"></script>
</body>
</html>`, metaBase, seo.headTags(), cssHref, bodyAttrs, nav, placeholder, jsHref)
	return []byte(doc)
}

func (s *docServer) seoFor(title, description, path, base string) pageSEO {
	return pageSEO{
		Title:       title,
		Description: description,
		Path:        path,
		Base:        base,
		SiteOrigin:  s.siteOrigin,
	}
}

func (s *docServer) readMarkdown(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.docsRoot, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *docServer) prerenderDoc(name, base string) string {
	md, err := s.readMarkdown(name)
	if err != nil {
		return "<p>Loading…</p>"
	}
	htmlBody, err := renderMarkdownHTML(md, base)
	if err != nil {
		return "<p>Loading…</p>\n<!-- prerender failed: " + html.EscapeString(err.Error()) + " -->"
	}
	return htmlBody
}

func (s *docServer) readmeHTML(base string) []byte {
	md, _ := s.readMarkdown("README.md")
	title := titleFromMarkdown(md, "Documentation")
	desc := descriptionFromMarkdown(md)
	seo := s.seoFor(title, desc, "/", base)
	return pageShell(seo, `data-page="readme"`, s.navLinks(base), s.prerenderDoc("README.md", base))
}

func (s *docServer) docPageHTML(name, base string) []byte {
	fallback := strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "_", " ")
	md, _ := s.readMarkdown(name)
	title := titleFromMarkdown(md, fallback)
	desc := descriptionFromMarkdown(md)
	seo := s.seoFor(title, desc, "/doc/"+docPageFile(name), base)
	attrs := fmt.Sprintf(`data-page="doc" data-md="%s"`, html.EscapeString(name))
	return pageShell(seo, attrs, s.navLinks(base), s.prerenderDoc(name, base))
}

func (s *docServer) diagramsIndexHTML(base string) []byte {
	names := s.listDiagrams()
	b, _ := json.Marshal(names)
	b64 := base64.StdEncoding.EncodeToString(b)
	attrs := fmt.Sprintf(`data-page="diagrams" data-diagrams-b64="%s"`, b64)

	var list strings.Builder
	list.WriteString(`<article class="markdown-body"><h2>Mermaid diagrams</h2>`)
	list.WriteString(`<p>Standalone diagram pages (sources in <code>docs/diagrams/</code>).</p><ul class="diagram-list">`)
	for _, n := range names {
		href := joinBase(base, "/diagrams/"+url.PathEscape(diagramPageFile(n)))
		fmt.Fprintf(&list, `<li><a href="%s">%s</a></li>`, html.EscapeString(href), html.EscapeString(n))
	}
	list.WriteString(`</ul></article>`)

	seo := s.seoFor("Diagrams", "Standalone Mermaid architecture diagrams for gdbforge.", "/diagrams/", base)
	return pageShell(seo, attrs, s.navLinks(base), list.String())
}

func (s *docServer) diagramPageHTML(name, base string) []byte {
	attrs := fmt.Sprintf(`data-page="diagram" data-diagram="%s"`, html.EscapeString(name))
	src, err := os.ReadFile(filepath.Join(s.docsRoot, "diagrams", name))
	placeholder := "<p>Loading…</p>"
	if err == nil {
		placeholder = fmt.Sprintf(
			`<article class="markdown-body"><h2>%s</h2><p><a href="%s">← Documentation index</a></p><pre><code>%s</code></pre></article>`,
			html.EscapeString(name),
			html.EscapeString(joinBase(base, "/")),
			html.EscapeString(string(src)),
		)
	}
	seo := s.seoFor(name, "Mermaid diagram: "+name+" (gdbforge documentation).", "/diagrams/"+diagramPageFile(name), base)
	return pageShell(seo, attrs, s.navLinks(base), placeholder)
}
