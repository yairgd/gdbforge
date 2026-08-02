// Package luacatalog embeds the shipped Lua script tree for gdbforge.
package luacatalog

import "embed"

// FS is the full lua/ catalog (scripts, sidecars, README).
//
//go:embed all:*
var FS embed.FS
