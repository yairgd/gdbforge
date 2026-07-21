package commands

// Completer returns name suggestions for a prefix.
// This is the completion interface used by trie rest-arg Tab (:b, :e, :layout)
// via CommandNode.CompleteArgs, and by GDB console Tab (-complete).
type Completer func(prefix string) []string
