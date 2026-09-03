//go:build !unix

package termui

func blockJobControlStop()           {}
func unblockJobControlStop()         {}
func stopForShellJobControl() error { return nil }
