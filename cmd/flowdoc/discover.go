package main

// Flow discovery is delegated to the official Go call-graph tool,
// golang.org/x/tools/cmd/callgraph (pinned as a `tool` directive in go.mod and
// invoked as `go tool callgraph`). flowdoc only parses its edge list, keeps the
// edges inside this module, and turns them into flow trees.

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const modulePrefix = "github.com/yairgd/gdbforge/"

type cgEdge struct {
	Caller cgFunc
	Callee cgFunc
}

type cgFunc struct {
	PkgPath string
	Recv    string
	Name    string
	Raw     string
}

func (f cgFunc) key() string {
	return f.PkgPath + "|" + f.Recv + "|" + f.Name
}

func (f cgFunc) display() string {
	if f.Recv != "" {
		return f.Recv + "." + f.Name
	}
	return shortPkgName(f.PkgPath) + "." + f.Name
}

func shortPkgName(pkgPath string) string {
	short := shortPkgPath(pkgPath)
	if i := strings.LastIndex(short, "/"); i >= 0 {
		return short[i+1:]
	}
	return short
}

// runCallgraph shells out to `go tool callgraph`, falling back to a callgraph
// binary on PATH. Returns edges whose caller and callee both live in this module.
func runCallgraph(root, algo string, pkgs []string) ([]cgEdge, error) {
	if algo == "" {
		algo = "vta"
	}
	if len(pkgs) == 0 {
		pkgs = []string{"./cmd/gdbforge"}
	}
	args := append([]string{"-algo", algo, "-format", "{{.Caller}}\t{{.Callee}}"}, pkgs...)

	candidates := [][]string{
		append([]string{"go", "tool", "callgraph"}, args...),
		append([]string{"callgraph"}, args...),
	}

	var lastErr error
	for _, cmdline := range candidates {
		if _, err := exec.LookPath(cmdline[0]); err != nil {
			lastErr = err
			continue
		}
		cmd := exec.Command(cmdline[0], cmdline[1:]...)
		cmd.Dir = root
		var out, errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("%s: %v: %s", strings.Join(cmdline[:2], " "), err, truncateErr(errBuf.String()))
			continue
		}
		edges := parseCallgraph(&out)
		if len(edges) == 0 {
			lastErr = fmt.Errorf("%s produced no in-module edges", strings.Join(cmdline[:2], " "))
			continue
		}
		fmt.Fprintf(os.Stderr, "flowdoc: callgraph(%s) → %d in-module edges\n", algo, len(edges))
		return edges, nil
	}
	return nil, fmt.Errorf("callgraph unavailable: %v", lastErr)
}

func truncateErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

func parseCallgraph(r *bytes.Buffer) []cgEdge {
	var edges []cgEdge
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
	seen := map[string]struct{}{}
	for sc.Scan() {
		line := sc.Text()
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		callerRaw, calleeRaw := line[:tab], line[tab+1:]
		if !strings.Contains(callerRaw, modulePrefix) || !strings.Contains(calleeRaw, modulePrefix) {
			continue
		}
		caller, ok := parseCGName(callerRaw)
		if !ok {
			continue
		}
		callee, ok := parseCGName(calleeRaw)
		if !ok {
			continue
		}
		if caller.key() == callee.key() {
			continue
		}
		k := caller.key() + ">" + callee.key()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		edges = append(edges, cgEdge{Caller: caller, Callee: callee})
	}
	return edges
}

// parseCGName splits callgraph's ssa.Function.String() form, e.g.
//
//	github.com/yairgd/gdbforge/internal/dlv.Complete
//	(*github.com/yairgd/gdbforge/cmd/gdbforge.DebuggerApp).handleInsertKey
func parseCGName(s string) (cgFunc, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "$") {
		return cgFunc{}, false
	}

	if strings.HasPrefix(s, "(") {
		close := strings.Index(s, ").")
		if close < 0 {
			return cgFunc{}, false
		}
		inner := s[1:close]
		name := s[close+2:]
		ptr := strings.HasPrefix(inner, "*")
		inner = strings.TrimPrefix(inner, "*")
		dot := strings.LastIndex(inner, ".")
		if dot < 0 {
			return cgFunc{}, false
		}
		pkgPath, typeName := inner[:dot], inner[dot+1:]
		recv := "(" + typeName + ")"
		if ptr {
			recv = "(*" + typeName + ")"
		}
		return cgFunc{PkgPath: pkgPath, Recv: recv, Name: name, Raw: s}, true
	}

	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return cgFunc{}, false
	}
	return cgFunc{PkgPath: s[:dot], Name: s[dot+1:], Raw: s}, true
}

type callAdj map[string][]cgFunc

func buildAdjacency(edges []cgEdge, include, skip []string) (callAdj, map[string]cgFunc) {
	adj := callAdj{}
	nodes := map[string]cgFunc{}
	for _, e := range edges {
		if !pathAllowed(e.Caller.PkgPath, include, skip) || !pathAllowed(e.Callee.PkgPath, include, skip) {
			continue
		}
		ck := e.Caller.key()
		adj[ck] = append(adj[ck], e.Callee)
		nodes[ck] = e.Caller
		nodes[e.Callee.key()] = e.Callee
	}
	for k := range adj {
		list := adj[k]
		sort.Slice(list, func(i, j int) bool {
			si, sj := calleeScore(list[i].Name), calleeScore(list[j].Name)
			if si != sj {
				return si > sj
			}
			return list[i].display() < list[j].display()
		})
		adj[k] = list
	}
	return adj, nodes
}

func discoverFlows(root string, spec discoverSpec, idx *funcIndex) ([]flow, error) {
	if !spec.Enabled {
		return nil, nil
	}
	if spec.MaxDepth < 1 {
		spec.MaxDepth = 4
	}
	if spec.MaxSteps < 2 {
		spec.MaxSteps = 18
	}
	if spec.MinSteps < 2 {
		spec.MinSteps = 3
	}
	if spec.Branching < 1 {
		spec.Branching = 3
	}

	edges, err := runCallgraph(root, spec.Algo, spec.AnalyzePackages)
	if err != nil {
		return nil, err
	}

	include := spec.IncludePrefixes
	if len(include) == 0 {
		include = []string{"cmd/gdbforge", "internal"}
	}
	adj, nodes := buildAdjacency(edges, include, spec.SkipPrefixes)

	entries := selectEntries(spec, adj, nodes)
	var flows []flow
	seenID := map[string]struct{}{}
	skippedNoLoc := 0

	for _, fn := range entries {
		steps := buildTree(fn, adj, idx, spec)
		if len(steps) < spec.MinSteps {
			continue
		}
		id := "auto-" + slug(strings.TrimPrefix(shortPkgPath(fn.PkgPath), "internal/")+"-"+recvSlug(fn.Recv)+"-"+fn.Name)
		if _, dup := seenID[id]; dup {
			continue
		}
		seenID[id] = struct{}{}

		root0 := steps[0]
		if root0.File == "" {
			skippedNoLoc++
			continue
		}

		flows = append(flows, flow{
			ID:    id,
			Title: fn.display() + " — call tree",
			Keywords: append([]string{
				"auto", "callgraph", strings.ToLower(fn.Name),
				shortPkgName(fn.PkgPath), shortPkgPath(fn.PkgPath),
			}, spec.Keywords...),
			Trigger: fmt.Sprintf("Static %s call tree rooted at %s (depth ≤ %d)",
				strings.ToUpper(orDefault(spec.Algo, "vta")), fn.Raw, spec.MaxDepth),
			Backend: []string{"both"},
			Steps:   steps,
			Auto:    true,
		})
		if spec.MaxFlows > 0 && len(flows) >= spec.MaxFlows {
			break
		}
	}

	sort.Slice(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })
	if skippedNoLoc > 0 {
		fmt.Fprintf(os.Stderr, "flowdoc: skipped %d entries without source location\n", skippedNoLoc)
	}
	return flows, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func recvSlug(recv string) string {
	recv = strings.NewReplacer("(", "", ")", "", "*", "").Replace(recv)
	return recv
}

func selectEntries(spec discoverSpec, adj callAdj, nodes map[string]cgFunc) []cgFunc {
	if len(spec.Entries) > 0 {
		var out []cgFunc
		for _, e := range spec.Entries {
			pkg := e.Pkg
			if pkg != "" && !strings.HasPrefix(pkg, "github.com/") {
				pkg = modulePrefix + strings.TrimPrefix(pkg, "/")
			}
			fn := cgFunc{PkgPath: pkg, Recv: normalizeRecv(e.Recv), Name: e.Name}
			fn.Raw = fn.display()
			out = append(out, fn)
		}
		return out
	}

	var keys []string
	for k, callees := range adj {
		if len(callees) == 0 {
			continue
		}
		fn := nodes[k]
		if !prefixMatch(shortPkgPath(fn.PkgPath), spec.EntryPrefixes) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]cgFunc, 0, len(keys))
	for _, k := range keys {
		out = append(out, nodes[k])
	}
	return out
}

func prefixMatch(short string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(short, p) {
			return true
		}
	}
	return false
}

func buildTree(rootFn cgFunc, adj callAdj, idx *funcIndex, spec discoverSpec) []step {
	var steps []step
	visited := map[string]struct{}{}

	var walk func(fn cgFunc, depth int)
	walk = func(fn cgFunc, depth int) {
		if len(steps) >= spec.MaxSteps || depth > spec.MaxDepth {
			return
		}
		key := fn.key()
		if _, dup := visited[key]; dup {
			return
		}
		visited[key] = struct{}{}

		file, line := locate(idx, fn)
		steps = append(steps, step{
			Symbol: fn.display(),
			File:   file,
			Line:   line,
			Indent: depth,
			Note:   noteFor(fn, depth),
		})

		callees := adj[key]
		emitted := 0
		for _, callee := range callees {
			if emitted >= spec.Branching || len(steps) >= spec.MaxSteps {
				break
			}
			if _, dup := visited[callee.key()]; dup {
				continue
			}
			before := len(steps)
			walk(callee, depth+1)
			if len(steps) > before {
				emitted++
			}
		}
	}

	walk(rootFn, 0)
	return steps
}

func noteFor(fn cgFunc, depth int) string {
	if depth == 0 {
		return "root (" + shortPkgPath(fn.PkgPath) + ")"
	}
	return shortPkgPath(fn.PkgPath)
}

func locate(idx *funcIndex, fn cgFunc) (string, int) {
	recvVariants := []string{fn.Recv}
	if fn.Recv != "" {
		recvVariants = append(recvVariants, normalizeRecv(fn.Recv))
	} else {
		recvVariants = append(recvVariants, "")
	}
	for _, recv := range recvVariants {
		ref, err := idx.resolve(chainSpec{
			Pkg:    fn.PkgPath,
			Recv:   recv,
			Name:   fn.Name,
			Symbol: fn.Name,
		})
		if err == nil && ref != nil {
			return ref.File, ref.Line
		}
	}
	for _, ref := range idx.all {
		if ref.Name == fn.Name && ref.PkgPath == fn.PkgPath {
			return ref.File, ref.Line
		}
	}
	return "", 0
}

func calleeScore(name string) int {
	l := strings.ToLower(name)
	score := 0
	for _, hint := range []string{
		"complete", "interrupt", "stopped", "refresh", "backend",
		"console", "debug", "break", "exec", "halt", "publish",
		"apply", "handle", "on", "sync", "activate", "send", "show",
	} {
		if strings.Contains(l, hint) {
			score += 2
		}
	}
	return score
}

func pathAllowed(pkgPath string, include, skip []string) bool {
	short := shortPkgPath(pkgPath)
	if !strings.HasPrefix(pkgPath, modulePrefix) {
		return false
	}
	for _, p := range skip {
		if strings.HasPrefix(short, p) {
			return false
		}
	}
	for _, p := range include {
		if strings.HasPrefix(short, p) {
			return true
		}
	}
	return false
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}
