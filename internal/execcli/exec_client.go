package execcli

import (
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// ExecClient runs an arbitrary command attached to a PTY.
type ExecClient struct {
	*ptyx.TTY
}

var _ core.Session = (*ExecClient)(nil)

// NewExecClient starts argv[0] with argv[1:] on a PTY and begins reading output.
func NewExecClient(argv []string) (*ExecClient, error) {
	tty, err := ptyx.Start(argv, ptyx.Options{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	return &ExecClient{TTY: tty}, nil
}
