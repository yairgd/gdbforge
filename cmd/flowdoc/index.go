package main

import (
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

type funcRef struct {
	PkgPath    string
	Recv       string
	Name       string
	File       string
	Line       int
	FullSymbol string
	TypesFunc  *types.Func
}

type funcIndex struct {
	byKey map[string]*funcRef
	byPos map[string]*funcRef
	all   []*funcRef
}

func loadFuncIndex(root string, patterns []string) (*funcIndex, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		patterns = []string{"./cmd/...", "./internal/..."}
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
		Dir: root,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages.Load returned errors")
	}

	idx := &funcIndex{byKey: map[string]*funcRef{}, byPos: map[string]*funcRef{}}
	for _, pkg := range pkgs {
		if pkg.IllTyped || pkg.Types == nil || pkg.Fset == nil {
			continue
		}
		for id, obj := range pkg.TypesInfo.Defs {
			if id.Name == "" {
				continue
			}
			fn, ok := obj.(*types.Func)
			if !ok {
				continue
			}
			pos := pkg.Fset.Position(fn.Pos())
			if pos.Filename == "" {
				continue
			}
			rel, err := filepath.Rel(root, pos.Filename)
			if err != nil {
				rel = pos.Filename
			}
			rel = filepath.ToSlash(rel)

			fname := fn.Name()
			recv, name := splitFuncName(fname)
			if recv == "" {
				recv = methodRecv(fn)
			}
			ref := &funcRef{
				PkgPath:    pkg.PkgPath,
				Recv:       recv,
				Name:       name,
				File:       rel,
				Line:       pos.Line,
				FullSymbol: displaySymbol(recv, name),
				TypesFunc:  fn,
			}
			idx.all = append(idx.all, ref)
			for _, k := range ref.keys() {
				if _, exists := idx.byKey[k]; !exists {
					idx.byKey[k] = ref
				}
			}
			posKey := fmt.Sprintf("%s:%d", ref.File, ref.Line)
			if _, exists := idx.byPos[posKey]; !exists {
				idx.byPos[posKey] = ref
			}
		}
	}
	if len(idx.all) == 0 {
		return nil, fmt.Errorf("no functions indexed under %v", patterns)
	}
	return idx, nil
}

func splitFuncName(full string) (recv, name string) {
	if strings.HasPrefix(full, "(") {
		if idx := strings.Index(full, ")."); idx > 0 {
			return full[:idx+1], full[idx+2:]
		}
	}
	return "", full
}

func methodRecv(fn *types.Func) string {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return ""
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return "(*" + n.Obj().Name() + ")"
	}
	return ""
}

func displaySymbol(recv, name string) string {
	if recv != "" {
		return recv + "." + name
	}
	return name
}

func (r *funcRef) keys() []string {
	shortPkg := shortPkgPath(r.PkgPath)
	var keys []string
	add := func(pkg, recv, name string) {
		keys = append(keys, fmt.Sprintf("%s|%s|%s", pkg, recv, name))
	}
	add(r.PkgPath, r.Recv, r.Name)
	add(shortPkg, r.Recv, r.Name)
	return keys
}

func shortPkgPath(full string) string {
	const mod = "github.com/yairgd/gdbforge/"
	if strings.HasPrefix(full, mod) {
		return strings.TrimPrefix(full, mod)
	}
	return full
}

func (idx *funcIndex) resolve(link chainSpec) (*funcRef, error) {
	pkg := strings.TrimSpace(link.Pkg)
	if pkg == "" {
		return nil, fmt.Errorf("chain step %q: missing pkg", link.Symbol)
	}
	if !strings.Contains(pkg, "/") && !strings.HasPrefix(pkg, "github.com/") {
		pkg = "github.com/yairgd/gdbforge/" + strings.TrimPrefix(pkg, "/")
	}
	recv := normalizeRecv(link.Recv)
	name := strings.TrimSpace(link.Name)
	if name == "" {
		names := symbolNames(link.Symbol)
		if len(names) == 0 {
			return nil, fmt.Errorf("chain step %q: missing name", link.Symbol)
		}
		name = names[0]
	}

	try := func(p string) (*funcRef, bool) {
		key := fmt.Sprintf("%s|%s|%s", p, recv, name)
		if ref, ok := idx.byKey[key]; ok {
			return ref, true
		}
		return nil, false
	}
	if ref, ok := try(pkg); ok {
		return ref, nil
	}
	if ref, ok := try(shortPkgPath(pkg)); ok {
		return ref, nil
	}

	var matches []*funcRef
	wantShort := shortPkgPath(pkg)
	for _, ref := range idx.all {
		if ref.Name != name {
			continue
		}
		if recv != "" && ref.Recv != recv && ref.Recv != "" {
			continue
		}
		if shortPkgPath(ref.PkgPath) == wantShort || strings.HasSuffix(ref.PkgPath, "/"+wantShort) {
			matches = append(matches, ref)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("chain step %q: function not found (pkg=%s recv=%s name=%s)", link.Symbol, pkg, recv, name)
	default:
		return nil, fmt.Errorf("chain step %q: ambiguous (%d matches for pkg=%s name=%s)", link.Symbol, len(matches), pkg, name)
	}
}

func normalizeRecv(recv string) string {
	recv = strings.TrimSpace(recv)
	if recv == "" {
		return ""
	}
	if !strings.HasPrefix(recv, "(") {
		recv = "(*" + strings.TrimPrefix(recv, "*") + ")"
	}
	return recv
}
