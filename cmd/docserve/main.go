// Command docserve serves xGDB documentation as HTML with Markdown and Mermaid rendering.
//
// Local HTTP server:
//
//	go run ./cmd/docserve
//	go run ./cmd/docserve --port 8765
//
// Static export (GitHub Pages):
//
//	go run ./cmd/docserve --export _site --base /gdbx/ --site-origin https://USER.github.io
//
// Preview exported site:
//
//	go run ./cmd/docserve --serve-static _site --port 8766
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	host := flag.String("host", "127.0.0.1", "bind address")
	port := flag.Int("port", 8765, "listen port")
	strictPort := flag.Bool("strict-port", false, "exit if port is already in use")
	exportDir := flag.String("export", "", "write static site to directory and exit")
	basePath := flag.String("base", "/", "URL base path for static export (e.g. /gdbx/)")
	siteOrigin := flag.String("site-origin", "", "absolute site origin for SEO (e.g. https://user.github.io)")
	serveStatic := flag.String("serve-static", "", "serve a previously exported static directory")
	flag.Parse()

	if *serveStatic != "" {
		runStaticServer(*host, *port, *strictPort, *serveStatic, *basePath)
		return
	}

	docsRoot, err := docsRootDir()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(docsRoot, "README.md")); err != nil {
		log.Fatalf("README.md not found under %s", docsRoot)
	}

	srv := &docServer{docsRoot: docsRoot, base: "/", siteOrigin: strings.TrimRight(*siteOrigin, "/")}

	if *exportDir != "" {
		if err := srv.export(*exportDir, *basePath); err != nil {
			log.Fatal(err)
		}
		log.Println(exportPreviewHint(*exportDir, *basePath))
		return
	}

	runDocServer(*host, *port, *strictPort, srv)
}

func runDocServer(host string, port int, strict bool, srv *docServer) {
	handler := http.HandlerFunc(srv.serveHTTP)

	ln, boundPort, err := listenWithFallback(host, port, strict)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	if boundPort != port {
		log.Printf("note: port %d in use, using %d instead", port, boundPort)
	}

	addr := fmt.Sprintf("http://%s:%d/", host, boundPort)
	log.Printf("xGDB docs: %s", addr)
	for _, name := range srv.listDocPages() {
		if name == "README.md" {
			continue
		}
		log.Printf("  %s: http://%s:%d/doc/%s", name, host, boundPort, url.PathEscape(name))
	}
	log.Printf("  Diagrams: http://%s:%d/diagrams", host, boundPort)
	log.Println("Press Ctrl+C to stop.")

	if err := http.Serve(ln, handler); err != nil {
		log.Fatal(err)
	}
}

func runStaticServer(host string, port int, strict bool, root, base string) {
	root = filepath.Clean(root)
	base = normalizeBase(base)
	if st, err := os.Stat(filepath.Join(root, "index.html")); err != nil || st.IsDir() {
		log.Fatalf("index.html not found under %s — run --export first", root)
	}

	ln, boundPort, err := listenWithFallback(host, port, strict)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	addr := fmt.Sprintf("http://%s:%d/", host, boundPort)
	if base != "/" {
		addr = fmt.Sprintf("http://%s:%d%s", host, boundPort, strings.TrimSuffix(base, "/")+"/")
	}
	log.Printf("Serving static docs from %s at %s", root, addr)
	log.Println("Press Ctrl+C to stop.")

	fileServer := http.FileServer(http.Dir(root))
	basePrefix := strings.TrimSuffix(base, "/")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if base != "/" {
			if path == basePrefix || path+"/" == base {
				http.Redirect(w, r, base, http.StatusFound)
				return
			}
			if !strings.HasPrefix(path, basePrefix+"/") && path != base {
				if path == "/" {
					http.Redirect(w, r, base, http.StatusFound)
					return
				}
				http.NotFound(w, r)
				return
			}
			path = strings.TrimPrefix(path, basePrefix)
			if path == "" {
				path = "/"
			}
			r.URL.Path = path
		}

		cleanPath := strings.TrimSuffix(r.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "/"
		}

		if cleanPath != "/" {
			localPath := filepath.Join(root, filepath.Clean(strings.TrimPrefix(cleanPath, "/")))
			if info, err := os.Stat(localPath); err == nil && info.IsDir() {
				index := filepath.Join(localPath, "index.html")
				if fileExists(index) {
					r.URL.Path = strings.TrimSuffix(r.URL.Path, "/") + "/index.html"
				}
			}
		}

		fileServer.ServeHTTP(w, r)
	})

	if err := http.Serve(ln, handler); err != nil {
		log.Fatal(err)
	}
}

func docsRootDir() (string, error) {
	if env := os.Getenv("CGDB_GO_DOCS_ROOT"); env != "" {
		return filepath.Clean(env), nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(wd, "docs"),
		filepath.Join(wd, "..", "docs"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "README.md")); err == nil && !st.IsDir() {
			return filepath.Clean(c), nil
		}
	}
	return filepath.Join(wd, "docs"), nil
}

type docServer struct {
	docsRoot   string
	base       string
	siteOrigin string
}

func (s *docServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/":
		s.sendHTML(w, s.readmeHTML(s.base))
	case strings.HasPrefix(path, "/doc/"):
		s.sendDocPage(w, strings.TrimPrefix(path, "/doc/"))
	case strings.HasSuffix(path, ".md") && strings.Count(path, "/") == 1:
		s.sendDocPage(w, strings.TrimPrefix(path, "/"))
	case path == "/diagrams":
		s.sendHTML(w, s.diagramsIndexHTML(s.base))
	case strings.HasPrefix(path, "/diagrams/"):
		name := strings.TrimPrefix(path, "/diagrams/")
		name, _ = url.PathUnescape(name)
		if name == "" {
			s.sendHTML(w, s.diagramsIndexHTML(s.base))
			return
		}
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(s.docsRoot, "diagrams", name)
		if !fileExists(p) {
			http.NotFound(w, r)
			return
		}
		s.sendHTML(w, s.diagramPageHTML(name, s.base))
	case strings.HasPrefix(path, "/raw/"):
		s.sendRaw(w, strings.TrimPrefix(path, "/raw/"))
	case strings.HasPrefix(path, "/www/"):
		s.sendStatic(w, strings.TrimPrefix(path, "/www/"))
	default:
		http.NotFound(w, r)
	}
}

func (s *docServer) listDocPages() []string {
	entries, err := os.ReadDir(s.docsRoot)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sortStrings(names)
	return names
}

func (s *docServer) listDiagrams() []string {
	dir := filepath.Join(s.docsRoot, "diagrams")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".mermaid") {
			names = append(names, e.Name())
		}
	}
	sortStrings(names)
	return names
}

func (s *docServer) sendHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *docServer) sendDocPage(w http.ResponseWriter, name string) {
	name, _ = url.PathUnescape(name)
	if strings.Contains(name, "/") || strings.Contains(name, "..") || !strings.HasSuffix(name, ".md") {
		http.NotFound(w, nil)
		return
	}
	found := false
	for _, n := range s.listDocPages() {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, nil)
		return
	}
	s.sendHTML(w, s.docPageHTML(name, s.base))
}

func (s *docServer) sendRaw(w http.ResponseWriter, rel string) {
	rel, _ = url.PathUnescape(rel)
	if strings.Contains(rel, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	filePath := filepath.Clean(filepath.Join(s.docsRoot, rel))
	if !strings.HasPrefix(filePath, filepath.Clean(s.docsRoot)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(filePath))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	if strings.HasSuffix(filePath, ".md") || strings.HasSuffix(filePath, ".mermaid") {
		ctype = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *docServer) sendStatic(w http.ResponseWriter, rel string) {
	if strings.Contains(rel, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	wwwRoot := filepath.Join(s.docsRoot, "www")
	filePath := filepath.Clean(filepath.Join(wwwRoot, rel))
	if !strings.HasPrefix(filePath, filepath.Clean(wwwRoot)) {
		http.NotFound(w, nil)
		return
	}
	f, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	defer f.Close()
	ctype := mime.TypeByExtension(filepath.Ext(filePath))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func listenWithFallback(host string, port int, strict bool) (net.Listener, int, error) {
	maxTries := 1
	if !strict {
		maxTries = 20
	}
	var lastErr error
	for candidate := port; candidate < port+maxTries; candidate++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, candidate))
		if err == nil {
			return ln, candidate, nil
		}
		lastErr = err
	}
	return nil, port, fmt.Errorf("could not bind %s ports %d-%d: %w", host, port, port+maxTries-1, lastErr)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func sortStrings(ss []string) {
	for i := 0; i < len(ss); i++ {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] < ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}
