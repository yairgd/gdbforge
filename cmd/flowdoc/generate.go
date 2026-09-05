package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateCatalog(root, specPath, outPath string) error {
	spec, err := loadSpec(specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}
	if spec.Repo == "" {
		spec.Repo = "yairgd/gdbforge"
	}
	if spec.Branch == "" {
		spec.Branch = "main"
	}

	idx, err := loadFuncIndex(root, spec.Packages)
	if err != nil {
		return fmt.Errorf("index functions: %w", err)
	}

	cat := catalog{
		Repo:   spec.Repo,
		Branch: spec.Branch,
		Flows:  make([]flow, 0, len(spec.Flows)),
	}

	curated := 0
	for _, fs := range spec.Flows {
		f, err := buildFlow(fs, idx)
		if err != nil {
			return fmt.Errorf("flow %q: %w", fs.ID, err)
		}
		cat.Flows = append(cat.Flows, f)
		curated++
	}

	discovered, err := discoverFlows(root, spec.Discover, idx)
	if err != nil {
		return fmt.Errorf("discover: %w", err)
	}
	cat.Flows = append(cat.Flows, discovered...)

	out, err := marshalCatalog(&cat)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "flowdoc: wrote %s (%d curated + %d auto = %d flows, %d functions indexed)\n",
		outPath, curated, len(discovered), len(cat.Flows), len(idx.all))
	return nil
}

func buildFlow(fs flowSpec, idx *funcIndex) (flow, error) {
	if fs.ID == "" {
		return flow{}, fmt.Errorf("missing id")
	}
	if len(fs.Chain) == 0 {
		return flow{}, fmt.Errorf("empty chain")
	}

	steps := make([]step, 0, len(fs.Chain))
	for i, link := range fs.Chain {
		ref, err := idx.resolve(link)
		if err != nil {
			return flow{}, err
		}
		sym := strings.TrimSpace(link.Symbol)
		if sym == "" {
			sym = ref.FullSymbol
		}
		indent := link.Indent
		if indent == 0 && i > 0 {
			indent = i
		}
		steps = append(steps, step{
			Symbol: sym,
			File:   ref.File,
			Line:   ref.Line,
			Indent: indent,
			Note:   link.Note,
			Branch: link.Branch,
		})
	}

	return flow{
		ID:          fs.ID,
		Title:       fs.Title,
		Keywords:    fs.Keywords,
		Trigger:     fs.Trigger,
		Backend:     fs.Backend,
		Steps:       steps,
		Breakpoints: fs.Breakpoints,
	}, nil
}
