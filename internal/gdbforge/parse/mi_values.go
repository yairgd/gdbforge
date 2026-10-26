package parse

import (
	"regexp"
	"strings"
)

var (
	miErrorRe    = regexp.MustCompile(`\^error,msg="((?:[^"\\]|\\.)*)"`)
	miValueRe    = regexp.MustCompile(`\^done,value="((?:[^"\\]|\\.)*)"`)
	miContentsRe = regexp.MustCompile(`contents="([0-9a-fA-F]*)"`)
	miRegNamesRe = regexp.MustCompile(`register-names=\[([^\]]*)\]`)
	miRegValueRe = regexp.MustCompile(`\{number="(\d+)",value="((?:[^"\\]|\\.)*)"\}`)
	miQuotedRe   = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
)

// MIUnquote undoes the C-string escapes of an MI string value.
func MIUnquote(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// MIError returns the message of the last ^error record in raw.
func MIError(raw string) (string, bool) {
	m := miErrorRe.FindAllStringSubmatch(raw, -1)
	if len(m) == 0 {
		return "", false
	}
	return MIUnquote(m[len(m)-1][1]), true
}

// MIEvaluateValue returns value from a -data-evaluate-expression reply.
func MIEvaluateValue(raw string) (string, bool) {
	m := miValueRe.FindAllStringSubmatch(raw, -1)
	if len(m) == 0 {
		return "", false
	}
	return MIUnquote(m[len(m)-1][1]), true
}

// MIMemoryHex joins the contents of every block of a -data-read-memory-bytes reply.
func MIMemoryHex(raw string) (string, bool) {
	if i := strings.LastIndex(raw, "^done,memory="); i >= 0 {
		raw = raw[i:]
	} else {
		return "", false
	}
	var b strings.Builder
	for _, m := range miContentsRe.FindAllStringSubmatch(raw, -1) {
		b.WriteString(m[1])
	}
	return b.String(), true
}

// MIRegisterNames returns the names (index = register number) of a -data-list-register-names reply.
func MIRegisterNames(raw string) []string {
	m := miRegNamesRe.FindAllStringSubmatch(raw, -1)
	if len(m) == 0 {
		return nil
	}
	var names []string
	for _, q := range miQuotedRe.FindAllStringSubmatch(m[len(m)-1][1], -1) {
		names = append(names, q[1])
	}
	return names
}

// MIRegisterValues maps register number to value for a -data-list-register-values reply.
func MIRegisterValues(raw string) map[int]string {
	if i := strings.LastIndex(raw, "^done,register-values="); i >= 0 {
		raw = raw[i:]
	}
	out := map[int]string{}
	for _, m := range miRegValueRe.FindAllStringSubmatch(raw, -1) {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		out[n] = MIUnquote(m[2])
	}
	return out
}

var miConsoleRe = regexp.MustCompile(`(?m)^~"((?:[^"\\]|\\.)*)"\s*$`)

// MIConsoleText joins the console stream records (~"...") of an MI reply into plain text,
// which is what a CLI command such as "info registers" prints in MI mode.
func MIConsoleText(raw string) string {
	var b strings.Builder
	for _, m := range miConsoleRe.FindAllStringSubmatch(strings.ReplaceAll(raw, "\r", ""), -1) {
		b.WriteString(MIUnquote(m[1]))
	}
	return b.String()
}

// InfoRegisters parses "info registers" text ("name 0xvalue natural...") into name -> hex value.
func InfoRegisters(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && strings.HasPrefix(f[1], "0x") {
			out[f[0]] = f[1]
		}
	}
	return out
}
