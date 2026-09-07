// Command flowdoc generates, validates, and refreshes docs/flows/flows.json.
//
// Source of truth: docs/flows/flows.spec.yaml (curated triggers + call chains).
// Auto flows use golang.org/x/tools/cmd/callgraph (`go tool callgraph -algo vta`).
//
//	go run ./cmd/flowdoc --generate
//	go run ./cmd/flowdoc --check
//	go run ./cmd/flowdoc --fix-lines
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type catalog struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Flows  []flow `json:"flows"`
}

type flow struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Keywords    []string `json:"keywords,omitempty"`
	Trigger     string   `json:"trigger,omitempty"`
	Backend     []string `json:"backend,omitempty"`
	Steps       []step   `json:"steps"`
	Breakpoints []string `json:"breakpoints,omitempty"`
	// Auto marks flows produced by the callgraph analyzer rather than curation.
	Auto bool `json:"auto,omitempty"`
}

type step struct {
	Symbol string `json:"symbol"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Indent int    `json:"indent,omitempty"`
	Note   string `json:"note,omitempty"`
	Branch string `json:"branch,omitempty"`
}

// marshalCatalog keeps small hand-curated catalogs readable and switches to
// compact JSON once discovery makes the file large (it is generated in CI).
func marshalCatalog(cat *catalog) ([]byte, error) {
	if len(cat.Flows) > 200 {
		return json.Marshal(cat)
	}
	out, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func main() {
	generate := flag.Bool("generate", false, "generate flows.json from flows.spec.yaml (go/packages)")
	check := flag.Bool("check", false, "validate flows.json (files exist, lines in range)")
	fixLines := flag.Bool("fix-lines", false, "refresh step line numbers from symbol (best-effort)")
	specPath := flag.String("spec", "docs/flows/flows.spec.yaml", "flow spec YAML (source of truth)")
	catalogPath := flag.String("catalog", "docs/flows/flows.json", "generated flow catalog JSON")
	repoRoot := flag.String("root", ".", "repository root for resolving step file paths")
	flag.Parse()

	root, err := filepath.Abs(*repoRoot)
	if err != nil {
		exitErr("repo root: %v", err)
	}
	catFile, err := filepath.Abs(*catalogPath)
	if err != nil {
		exitErr("catalog path: %v", err)
	}
	specFile, err := filepath.Abs(*specPath)
	if err != nil {
		exitErr("spec path: %v", err)
	}

	if *generate {
		if err := generateCatalog(root, specFile, catFile); err != nil {
			exitErr("%v", err)
		}
	}

	if !*generate && !*check && !*fixLines {
		*check = true
	}

	if *check || *fixLines {
		data, err := os.ReadFile(catFile)
		if err != nil {
			exitErr("read catalog: %v", err)
		}
		var cat catalog
		if err := json.Unmarshal(data, &cat); err != nil {
			exitErr("parse JSON: %v", err)
		}

		if *fixLines {
			changed := fixCatalogLines(&cat, root)
			if changed {
				out, err := marshalCatalog(&cat)
				if err != nil {
					exitErr("marshal JSON: %v", err)
				}
				if err := os.WriteFile(catFile, out, 0o644); err != nil {
					exitErr("write catalog: %v", err)
				}
				fmt.Fprintf(os.Stderr, "flowdoc: updated %s\n", catFile)
			}
		}

		if *check {
			if err := validateCatalog(&cat, root); err != nil {
				exitErr("%v", err)
			}
			fmt.Fprintf(os.Stderr, "flowdoc: OK (%d flows)\n", len(cat.Flows))
		}
	}
}

func validateCatalog(cat *catalog, root string) error {
	seen := map[string]struct{}{}
	var errs []string

	for _, f := range cat.Flows {
		if f.ID == "" {
			errs = append(errs, "flow missing id")
			continue
		}
		if _, ok := seen[f.ID]; ok {
			errs = append(errs, fmt.Sprintf("flow %q: duplicate id", f.ID))
		}
		seen[f.ID] = struct{}{}
		if f.Title == "" {
			errs = append(errs, fmt.Sprintf("flow %q: missing title", f.ID))
		}
		if len(f.Steps) == 0 {
			errs = append(errs, fmt.Sprintf("flow %q: no steps", f.ID))
		}
		for i, st := range f.Steps {
			if st.File == "" {
				continue
			}
			path := filepath.Join(root, st.File)
			if _, err := os.Stat(path); err != nil {
				errs = append(errs, fmt.Sprintf("flow %q step %d: file %q: %v", f.ID, i, st.File, err))
				continue
			}
			if st.Line < 1 {
				errs = append(errs, fmt.Sprintf("flow %q step %d: %s:%d line out of range", f.ID, i, st.File, st.Line))
				continue
			}
			lineCount, err := countLines(path)
			if err != nil {
				errs = append(errs, fmt.Sprintf("flow %q step %d: %s: %v", f.ID, i, st.File, err))
				continue
			}
			if st.Line > lineCount {
				errs = append(errs, fmt.Sprintf("flow %q step %d: %s:%d exceeds file length (%d)", f.ID, i, st.File, st.Line, lineCount))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func fixCatalogLines(cat *catalog, root string) bool {
	changed := false
	for fi := range cat.Flows {
		for si := range cat.Flows[fi].Steps {
			st := &cat.Flows[fi].Steps[si]
			if st.File == "" {
				continue
			}
			path := filepath.Join(root, st.File)
			line, ok := findSymbolLine(path, st.Symbol)
			if !ok || line == st.Line {
				continue
			}
			st.Line = line
			changed = true
			fmt.Fprintf(os.Stderr, "flowdoc: %s step %q -> %s:%d\n", cat.Flows[fi].ID, st.Symbol, st.File, line)
		}
	}
	return changed
}

func findSymbolLine(path, symbol string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(string(data), "\n")
	names := symbolNames(symbol)
	for _, name := range names {
		pat := regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s+)?` + regexp.QuoteMeta(name) + `\s*\(`)
		for i, line := range lines {
			if pat.MatchString(line) {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func symbolNames(symbol string) []string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil
	}
	parts := strings.FieldsFunc(symbol, func(r rune) bool {
		return r == '/' || r == '→'
	})
	var names []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if idx := strings.LastIndex(p, "."); idx >= 0 {
			p = p[idx+1:]
		}
		names = append(names, p)
	}
	return names
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	n := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		n++
	}
	return n, nil
}

func exitErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "flowdoc: "+format+"\n", args...)
	os.Exit(1)
}
