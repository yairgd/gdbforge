//go:build !unix

package termui

func flushControllingTTYInput() {}
