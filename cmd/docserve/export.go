package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (s *docServer) export(outDir, base string) error {
	base = normalizeBase(base)
	outDir = filepath.Clean(outDir)

	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clean output dir: %w", err)
	}

	dirs := []string{
		outDir,
		filepath.Join(outDir, "doc"),
		filepath.Join(outDir, "raw"),
		filepath.Join(outDir, "raw", "diagrams"),
		filepath.Join(outDir, "www"),
		filepath.Join(outDir, "diagrams"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(outDir, ".nojekyll"), nil, 0o644); err != nil {
		return fmt.Errorf("write .nojekyll: %w", err)
	}

	if err := copyTree(filepath.Join(s.docsRoot, "www"), filepath.Join(outDir, "www")); err != nil {
		return fmt.Errorf("copy www: %w", err)
	}

	for _, name := range s.listDocPages() {
		src := filepath.Join(s.docsRoot, name)
		dst := filepath.Join(outDir, "raw", name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy raw %s: %w", name, err)
		}
	}

	for _, name := range s.listDiagrams() {
		src := filepath.Join(s.docsRoot, "diagrams", name)
		dst := filepath.Join(outDir, "raw", "diagrams", name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy diagram source %s: %w", name, err)
		}
	}

	if err := os.WriteFile(filepath.Join(outDir, "index.html"), s.readmeHTML(base), 0o644); err != nil {
		return fmt.Errorf("write index.html: %w", err)
	}

	for _, name := range s.listDocPages() {
		if name == "README.md" {
			continue
		}
		dst := filepath.Join(outDir, "doc", name)
		if err := os.WriteFile(dst, s.docPageHTML(name, base), 0o644); err != nil {
			return fmt.Errorf("write doc page %s: %w", name, err)
		}
	}

	if err := os.WriteFile(filepath.Join(outDir, "diagrams", "index.html"), s.diagramsIndexHTML(base), 0o644); err != nil {
		return fmt.Errorf("write diagrams index: %w", err)
	}

	for _, name := range s.listDiagrams() {
		dst := filepath.Join(outDir, "diagrams", name)
		if err := os.WriteFile(dst, s.diagramPageHTML(name, base), 0o644); err != nil {
			return fmt.Errorf("write diagram page %s: %w", name, err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyTree(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func exportPreviewHint(outDir, base string) string {
	base = normalizeBase(base)
	if base == "/" {
		return fmt.Sprintf("Static site written to %s — preview: go run ./cmd/docserve --serve-static %s", outDir, outDir)
	}
	return fmt.Sprintf("Static site written to %s — preview: go run ./cmd/docserve --serve-static %s --base %s (open http://127.0.0.1:8766%s)", outDir, outDir, base, base)
}
