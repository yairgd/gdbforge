package persist

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// CmdlineHistoryFile is the YAML filename under DirName.
	CmdlineHistoryFile = "cmdline_history.yaml"
	// CmdlineHistoryMax is the max entries kept per list (oldest dropped).
	CmdlineHistoryMax = 500
)

// CmdlineHistoryDoc is the on-disk YAML document for CmdWidget history.
type CmdlineHistoryDoc struct {
	Commands []string `yaml:"commands"`
	Search   []string `yaml:"search"`
}

// CmdlineHistoryPath returns dir/.gdbforge/cmdline_history.yaml.
func CmdlineHistoryPath(dir string) string {
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, DirName, CmdlineHistoryFile)
}

func capHistory(items []string, max int) []string {
	if max <= 0 {
		max = CmdlineHistoryMax
	}
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1] == s {
			continue
		}
		out = append(out, s)
	}
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// SaveCmdlineHistory writes command and search history under dir/.gdbforge/.
// Creates the directory if needed. Empty lists still write a valid file.
func SaveCmdlineHistory(dir string, commands, search []string) error {
	doc := CmdlineHistoryDoc{
		Commands: capHistory(commands, CmdlineHistoryMax),
		Search:   capHistory(search, CmdlineHistoryMax),
	}
	if doc.Commands == nil {
		doc.Commands = []string{}
	}
	if doc.Search == nil {
		doc.Search = []string{}
	}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal cmdline history: %w", err)
	}
	outDir := filepath.Join(dir, DirName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	path := CmdlineHistoryPath(dir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// LoadCmdlineHistory reads dir/.gdbforge/cmdline_history.yaml.
// Missing file returns (nil, nil, nil).
func LoadCmdlineHistory(dir string) (commands, search []string, err error) {
	path := CmdlineHistoryPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var doc CmdlineHistoryDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return capHistory(doc.Commands, CmdlineHistoryMax), capHistory(doc.Search, CmdlineHistoryMax), nil
}
