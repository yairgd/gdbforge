package main

import (
	"bytes"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

type pageSEO struct {
	Title       string
	Description string
	Path        string // site path, e.g. "/" or "/doc/ARCHITECTURE.md"
	Base        string
	SiteOrigin  string // e.g. https://user.github.io — empty skips absolute URLs
}

func (p pageSEO) absoluteURL() string {
	if p.SiteOrigin == "" {
		return ""
	}
	origin := strings.TrimRight(p.SiteOrigin, "/")
	return origin + joinBase(p.Base, p.Path)
}

func (p pageSEO) headTags() string {
	title := html.EscapeString(p.Title + " — gdbforge Docs")
	desc := html.EscapeString(p.Description)
	if desc == "" {
		desc = html.EscapeString("gdbforge documentation: Vim-inspired terminal UI framework and GDB debugger in Go.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "  <title>%s</title>\n", title)
	fmt.Fprintf(&b, "  <meta name=\"description\" content=\"%s\">\n", desc)
	fmt.Fprintf(&b, "  <meta name=\"robots\" content=\"index,follow\">\n")

	canon := p.absoluteURL()
	if canon != "" {
		fmt.Fprintf(&b, "  <link rel=\"canonical\" href=\"%s\">\n", html.EscapeString(canon))
		fmt.Fprintf(&b, "  <meta property=\"og:url\" content=\"%s\">\n", html.EscapeString(canon))
	}
	fmt.Fprintf(&b, "  <meta property=\"og:type\" content=\"website\">\n")
	fmt.Fprintf(&b, "  <meta property=\"og:site_name\" content=\"gdbforge Docs\">\n")
	fmt.Fprintf(&b, "  <meta property=\"og:title\" content=\"%s\">\n", title)
	fmt.Fprintf(&b, "  <meta property=\"og:description\" content=\"%s\">\n", desc)
	fmt.Fprintf(&b, "  <meta name=\"twitter:card\" content=\"summary\">\n")
	fmt.Fprintf(&b, "  <meta name=\"twitter:title\" content=\"%s\">\n", title)
	fmt.Fprintf(&b, "  <meta name=\"twitter:description\" content=\"%s\">\n", desc)
	return b.String()
}

var headingRE = regexp.MustCompile(`(?m)^#\s+(.+)$`)

func titleFromMarkdown(md, fallback string) string {
	if m := headingRE.FindStringSubmatch(md); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return fallback
}

func descriptionFromMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var parts []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "```") ||
			strings.HasPrefix(trim, "---") || strings.HasPrefix(trim, "|") ||
			strings.HasPrefix(trim, ">") || strings.HasPrefix(trim, "- ") ||
			strings.HasPrefix(trim, "* ") {
			if len(parts) > 0 {
				break
			}
			continue
		}
		parts = append(parts, trim)
		if len(strings.Join(parts, " ")) > 40 {
			break
		}
	}
	text := strings.Join(parts, " ")
	text = collapseSpace(text)
	text = stripInlineMarkdown(text)
	if text == "" {
		return ""
	}
	const max = 155
	runes := []rune(text)
	if len(runes) > max {
		return string(runes[:max-1]) + "…"
	}
	return text
}

func stripInlineMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	re := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	s = re.ReplaceAllString(s, "$1")
	return collapseSpace(s)
}

func collapseSpace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func renderMarkdownHTML(md string, base string) (string, error) {
	md = rewriteDocLinks(md, base)
	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return `<article class="markdown-body">` + buf.String() + `</article>`, nil
}

// rewriteDocLinks turns relative .md / diagrams links into site paths for static HTML.
func rewriteDocLinks(md, base string) string {
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	return re.ReplaceAllStringFunc(md, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		label, href := sub[1], sub[2]
		hash := ""
		if i := strings.Index(href, "#"); i >= 0 {
			hash = href[i:]
			href = href[:i]
		}
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
			strings.HasPrefix(href, "mailto:") || href == "" {
			return m
		}
		name := filepath.Base(href)
		switch {
		case strings.HasSuffix(href, ".md"):
			if name == "README.md" {
				href = joinBase(base, "/")
			} else {
				href = joinBase(base, "/doc/"+docPageFile(name))
			}
		case strings.Contains(href, "diagrams/") && strings.HasSuffix(href, ".mermaid"):
			href = joinBase(base, "/diagrams/"+diagramPageFile(name))
		default:
			return m
		}
		return fmt.Sprintf("[%s](%s%s)", label, href, hash)
	})
}

func robotsTxt(siteOrigin, base string) string {
	base = normalizeBase(base)
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	b.WriteByte('\n')
	if siteOrigin != "" {
		origin := strings.TrimRight(siteOrigin, "/")
		fmt.Fprintf(&b, "Sitemap: %s%ssitemap.xml\n", origin, base)
	}
	return b.String()
}

func sitemapXML(siteOrigin, base string, paths []string) string {
	origin := strings.TrimRight(siteOrigin, "/")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range paths {
		loc := origin + joinBase(base, p)
		fmt.Fprintf(&b, "  <url><loc>%s</loc></url>\n", html.EscapeString(loc))
	}
	b.WriteString("</urlset>\n")
	return b.String()
}
