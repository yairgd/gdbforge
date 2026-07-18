package mcp

import (
	"regexp"
	"strings"
)

var fullnameRe = regexp.MustCompile(`fullname="((?:\\.|[^"\\])*)"`)

// ParseSourceFileList extracts fullname paths from -file-list-exec-source-files output.
func ParseSourceFileList(raw string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, m := range fullnameRe.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		path := strings.ReplaceAll(m[1], `\"`, `"`)
		path = strings.ReplaceAll(path, `\\`, `\`)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
