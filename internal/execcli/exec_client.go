package execcli

import (
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/ptyx"
)

// ExecClient runs an arbitrary command attached to a PTY.
type ExecClient struct {
	*ptyx.Client
}

var _ core.Session = (*ExecClient)(nil)

// NewExecClient starts argv[0] with argv[1:] on a PTY and begins reading output.
func NewExecClient(argv []string) (*ExecClient, error) {
	pty, err := ptyx.New(argv, ptyx.Options{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	return &ExecClient{Client: pty}, nil
}
