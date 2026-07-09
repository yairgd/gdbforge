package termui

import (
	"fmt"
	"strings"

	"github.com/yairgd/cgdb-go/internal/platform"
)

// LogCompletionPresenter writes completion candidates to a logger.
type LogCompletionPresenter struct {
	log *platform.NamedLogger
}

func NewLogCompletionPresenter(log *platform.NamedLogger) *LogCompletionPresenter {
	return &LogCompletionPresenter{log: log}
}

func (p *LogCompletionPresenter) Show(result CompletionResult) {
	if p.log == nil {
		return
	}

	if len(result.Names) == 0 {
		p.log.Info("completions: (none)")
		return
	}

	p.log.Info(fmt.Sprintf("completions: %s", strings.Join(result.Names, "  ")))
}
