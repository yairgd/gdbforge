package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
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

func (s *docServer) navLinks(base string) string {
	parts := []string{fmt.Sprintf(`<a href="%s">Index</a>`, html.EscapeString(joinBase(base, "/")))}
	for _, name := range s.listDocPages() {
		if name == "README.md" {
			continue
		}
		label := strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "_", " ")
		href := joinBase(base, "/doc/"+url.PathEscape(name))
		parts = append(parts, fmt.Sprintf(`<a href="%s">%s</a>`, href, html.EscapeString(label)))
	}
	parts = append(parts, fmt.Sprintf(`<a href="%s">Diagrams</a>`, html.EscapeString(joinBase(base, "/diagrams/"))))
	return strings.Join(parts, "\n      ")
}

func pageShell(title, bodyAttrs, nav, placeholder, base string) []byte {
	if placeholder == "" {
		placeholder = "<p>Loading…</p>"
	}
	base = normalizeBase(base)
	metaBase := html.EscapeString(base)
	cssHref := html.EscapeString(joinBase(base, "/www/site.css"))
	jsHref := html.EscapeString(joinBase(base, "/www/docs.js"))
	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="docs-base" content="%s">
  <title>%s — cgdb-go Docs</title>
  <link rel="stylesheet" href="%s">
</head>
<body %s>
  <header>
    <h1>cgdb-go Documentation</h1>
    <nav>
      %s
    </nav>
  </header>
  <main id="content">%s</main>
  <footer>cgdb-go docs · Markdown + Mermaid via CDN</footer>
  <script src="%s"></script>
</body>
</html>`, metaBase, html.EscapeString(title), cssHref, bodyAttrs, nav, placeholder, jsHref)
	return []byte(doc)
}

func (s *docServer) readmeHTML(base string) []byte {
	return pageShell("Documentation", `data-page="readme"`, s.navLinks(base), "", base)
}

func (s *docServer) docPageHTML(name, base string) []byte {
	title := strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "_", " ")
	attrs := fmt.Sprintf(`data-page="doc" data-md="%s"`, html.EscapeString(name))
	return pageShell(title, attrs, s.navLinks(base), "", base)
}

func (s *docServer) diagramsIndexHTML(base string) []byte {
	names := s.listDiagrams()
	b, _ := json.Marshal(names)
	b64 := base64.StdEncoding.EncodeToString(b)
	attrs := fmt.Sprintf(`data-page="diagrams" data-diagrams-b64="%s"`, b64)
	return pageShell("Diagrams", attrs, s.navLinks(base), "", base)
}

func (s *docServer) diagramPageHTML(name, base string) []byte {
	attrs := fmt.Sprintf(`data-page="diagram" data-diagram="%s"`, html.EscapeString(name))
	return pageShell(name, attrs, s.navLinks(base), "", base)
}
