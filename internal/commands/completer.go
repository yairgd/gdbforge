package commands

// Completer returns name suggestions for a prefix.
// trailingSpace is true when the cmdline cursor sits on a space after the rest token
// (CommandParser Sync uses cursor-1, so prefix never ends with space).
type Completer func(prefix string, trailingSpace bool) []string
