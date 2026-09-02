package debugger

// InferiorMode selects where the debuggee's stdin/stdout/stderr are routed.
type InferiorMode int

const (
	InferiorInternal InferiorMode = iota // in-app IO pane
	InferiorExternal                     // external terminal / pts path
)

func (m InferiorMode) String() string {
	switch m {
	case InferiorExternal:
		return "external"
	default:
		return "internal"
	}
}

// InferiorIO is the program stdio routing surface (internal IO pane vs external tty).
type InferiorIO interface {
	InferiorMode() InferiorMode
	InferiorTTYPath() string
	SetInferiorInternal() error
	SetInferiorExternal() error
	SetInferiorPath(path string) error
}
