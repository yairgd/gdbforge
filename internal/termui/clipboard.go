package termui

// ClipboardIO is the shared copy/paste bridge used by Viewport-backed widgets.
type ClipboardIO struct {
	Copy  func(text string)
	Paste func() string
}

func (c *ClipboardIO) copyText(text string) {
	if c == nil || c.Copy == nil || text == "" {
		return
	}
	c.Copy(text)
}

func (c *ClipboardIO) pasteText() string {
	if c == nil || c.Paste == nil {
		return ""
	}
	return c.Paste()
}
