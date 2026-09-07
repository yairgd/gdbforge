//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package termui

func stopForShellJobControl() error { return nil }
