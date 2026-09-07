package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type flowSpecFile struct {
	Repo     string       `yaml:"repo"`
	Branch   string       `yaml:"branch"`
	Packages []string     `yaml:"packages"`
	Flows    []flowSpec   `yaml:"flows"`
	Discover discoverSpec `yaml:"discover"`
}

// discoverSpec drives automatic flow discovery. The analysis itself is done by
// golang.org/x/tools/cmd/callgraph (`go tool callgraph`); these knobs only
// control which roots become flows and how deep each tree is rendered.
type discoverSpec struct {
	Enabled bool   `yaml:"enabled"`
	Algo    string `yaml:"algo"` // static | cha | rta | vta (callgraph -algo)

	AnalyzePackages []string `yaml:"analyze_packages"`
	EntryPrefixes   []string `yaml:"entry_prefixes"`
	IncludePrefixes []string `yaml:"include_prefixes"`
	SkipPrefixes    []string `yaml:"skip_prefixes"`

	MaxDepth  int `yaml:"max_depth"`
	MaxSteps  int `yaml:"max_steps"`
	MinSteps  int `yaml:"min_steps"`
	Branching int `yaml:"branching"`
	MaxFlows  int `yaml:"max_flows"`

	Keywords []string        `yaml:"keywords"`
	Entries  []discoverEntry `yaml:"entries"`
}

type discoverEntry struct {
	ID       string   `yaml:"id"`
	Pkg      string   `yaml:"pkg"`
	Recv     string   `yaml:"recv"`
	Name     string   `yaml:"name"`
	Title    string   `yaml:"title"`
	Trigger  string   `yaml:"trigger"`
	Keywords []string `yaml:"keywords"`
}

type flowSpec struct {
	ID          string      `yaml:"id"`
	Title       string      `yaml:"title"`
	Keywords    []string    `yaml:"keywords"`
	Trigger     string      `yaml:"trigger"`
	Backend     []string    `yaml:"backend"`
	Chain       []chainSpec `yaml:"chain"`
	Breakpoints []string    `yaml:"breakpoints"`
}

type chainSpec struct {
	Symbol string `yaml:"symbol"`
	Pkg    string `yaml:"pkg"`
	Recv   string `yaml:"recv"`
	Name   string `yaml:"name"`
	Indent int    `yaml:"indent"`
	Note   string `yaml:"note"`
	Branch string `yaml:"branch"`
}

func loadSpec(path string) (*flowSpecFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec flowSpecFile
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
