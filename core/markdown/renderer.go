package markdown

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/v2"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

// Renderer handles Markdown rendering.
type Renderer struct {
	md goldmark.Markdown
}

// NewRenderer creates a new Markdown renderer with extensions.
func NewRenderer() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (Table, Strikethrough, TaskList, Autolink)
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // Allow raw HTML (needed for some Mermaid scenarios or user embedded HTML)
		),
	)

	return &Renderer{
		md: md,
	}
}

// Render converts Markdown to HTML. YAML frontmatter (--- ... ---) is stripped
// before rendering so it does not appear as an <hr> in the output.
func (r *Renderer) Render(source []byte) ([]byte, error) {
	_, body, _ := ParseFrontmatter(source)
	var buf bytes.Buffer
	if err := r.md.Convert(body, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
