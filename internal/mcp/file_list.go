package mcp

import (
	"strings"
)

// ParseSourceFileList extracts paths from -file-list-exec-source-files output.
// Only the files=[...] payload is considered (ignores fullname= on *stopped /
// stack frames that may share the same PTY capture). Prefers fullname over file.
func ParseSourceFileList(raw string) []string {
	payload, ok := miListPayload(raw, "files")
	if !ok {
		return nil
	}

	seen := make(map[string]struct{})
	var out []string
	for _, chunk := range miBraceChunks(payload) {
		path := extractQuotedField(chunk, "fullname")
		if path == "" {
			path = extractQuotedField(chunk, "file")
		}
		path = unescapeMI(path)
		if path == "" {
			continue
		}
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

// miListPayload returns the interior of key=[...] from an MI result, scanning
// nested brackets so nested {..} values do not truncate the list early.
func miListPayload(raw, key string) (string, bool) {
	marker := key + "=["
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return "", false
	}
	i := idx + len(marker)
	depth := 1
	for i < len(raw) {
		switch raw[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return raw[idx+len(marker) : i], true
			}
		case '"':
			// Skip MI quoted strings so brackets inside paths are ignored.
			i++
			for i < len(raw) {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '"' {
					break
				}
				i++
			}
		}
		i++
	}
	return "", false
}

// miBraceChunks returns each top-level {...} object inside s.
func miBraceChunks(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		start := i
		depth := 0
		for i < len(s) {
			switch s[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					out = append(out, s[start:i+1])
					goto next
				}
			case '"':
				i++
				for i < len(s) {
					if s[i] == '\\' {
						i += 2
						continue
					}
					if s[i] == '"' {
						break
					}
					i++
				}
			}
			i++
		}
		return out
	next:
	}
	return out
}
