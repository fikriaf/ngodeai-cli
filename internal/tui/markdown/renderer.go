package markdown

import (
	"github.com/charmbracelet/glamour"
)

// Renderer handles markdown to terminal rendering
type Renderer struct {
	renderer *glamour.TermRenderer
}

// NewRenderer creates a markdown renderer with the specified theme
func NewRenderer(themeName string) (*Renderer, error) {
	var style string

	// Map our theme names to glamour styles
	switch themeName {
	case "dracula":
		style = "dracula"
	case "catppuccin":
		style = "catppuccin"
	case "tokyonight":
		style = "tokyo-night"
	case "gruvbox":
		style = "gruvbox"
	case "monokai":
		style = "monokai"
	case "onedark":
		style = "onedark"
	case "default":
		style = "dark"
	default:
		style = "dark"
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(80),
	)

	if err != nil {
		return nil, err
	}

	return &Renderer{
		renderer: renderer,
	}, nil
}

// Render converts markdown text to styled terminal output
func (r *Renderer) Render(markdown string) (string, error) {
	return r.renderer.Render(markdown)
}
