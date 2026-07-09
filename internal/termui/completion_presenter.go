package termui

// CompletionResult is passed to a CompletionPresenter when the user requests
// tab completion in command mode.
type CompletionResult struct {
	Input string
	Token string
	Names []string
}

// CompletionPresenter displays command completion candidates. Implementations
// may write to a log panel, show a popup, or use any other UI affordance.
type CompletionPresenter interface {
	Show(result CompletionResult)
}
