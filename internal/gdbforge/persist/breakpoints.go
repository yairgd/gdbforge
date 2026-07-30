package persist

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"

	"gopkg.in/yaml.v3"
)

const (
	// DirName is the project-local config directory (cwd, usually the build dir).
	DirName = ".gdbforge"
	// BreakpointsFile is the YAML filename under DirName.
	BreakpointsFile = "breakpoints.yaml"
)

// BreakpointEntry is one persisted breakpoint row.
type BreakpointEntry struct {
	File      string `yaml:"file"`
	Line      int    `yaml:"line"`
	Enabled   bool   `yaml:"enabled"`
	Condition string `yaml:"condition,omitempty"`
}

// BreakpointsDoc is the on-disk YAML document.
type BreakpointsDoc struct {
	Breakpoints []BreakpointEntry `yaml:"breakpoints"`
}

// BreakpointsPath returns dir/.gdbforge/breakpoints.yaml.
func BreakpointsPath(dir string) string {
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, DirName, BreakpointsFile)
}

// SaveBreakpoints writes items as YAML under dir/.gdbforge/.
// Creates the directory if needed. An empty list still writes a valid file.
func SaveBreakpoints(dir string, items []models.BreakInfo) error {
	doc := BreakpointsDoc{Breakpoints: make([]BreakpointEntry, 0, len(items))}
	for _, it := range items {
		if it.File == "" || it.Line < 1 {
			continue
		}
		doc.Breakpoints = append(doc.Breakpoints, BreakpointEntry{
			File:      it.File,
			Line:      it.Line,
			Enabled:   it.Enabled,
			Condition: it.Condition,
		})
	}
	if err := writeYAMLAtomic(BreakpointsPath(dir), &doc); err != nil {
		return fmt.Errorf("breakpoints: %w", err)
	}
	return nil
}

// LoadBreakpoints reads dir/.gdbforge/breakpoints.yaml.
// Missing file returns (nil, nil).
func LoadBreakpoints(dir string) ([]models.BreakInfo, error) {
	path := BreakpointsPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var doc BreakpointsDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]models.BreakInfo, 0, len(doc.Breakpoints))
	for _, e := range doc.Breakpoints {
		if e.File == "" || e.Line < 1 {
			continue
		}
		out = append(out, models.BreakInfo{
			File:      e.File,
			Line:      e.Line,
			Enabled:   e.Enabled,
			Condition: e.Condition,
		})
	}
	return out, nil
}
