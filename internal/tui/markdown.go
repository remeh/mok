package tui

import (
	"github.com/charmbracelet/glamour"
)

// markdownRenderer wraps a glamour TermRenderer for reuse.
type markdownRenderer struct {
	renderer *glamour.TermRenderer
}

// newMarkdownRenderer creates a renderer configured for terminal output.
func newMarkdownRenderer(width int) (*markdownRenderer, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-2),
	)
	if err != nil {
		return nil, err
	}
	return &markdownRenderer{renderer: r}, nil
}

// Render converts markdown text to styled terminal text.
func (mr *markdownRenderer) Render(md string) (string, error) {
	return mr.renderer.Render(md)
}
