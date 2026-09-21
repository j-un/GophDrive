package markdown

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v3"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

// Renderer handles Markdown rendering.
type Renderer struct {
	parser parser.Parser
	html   html.Renderer
}

// NewRenderer creates a new Markdown renderer with extensions.
func NewRenderer() *Renderer {
	p := parser.New(
		parser.WithAutoHeadingID(),
		parser.WithExtensions(
			extension.GFMParser, // Table, Strikethrough, TaskList, Autolink
			highlighting.Parser,
		),
	)
	r := html.New(
		html.WithHardWraps(),
		html.WithXHTML(),
		html.WithUnsafe(), // Allow raw HTML (needed for some Mermaid scenarios or user embedded HTML)
		html.WithExtensions(
			extension.GFMHTMLRenderer,
			highlighting.NewHTMLRenderer(
				highlighting.WithStyle("github"),
				highlighting.WithFormatterOptions(
					chromahtml.WithClasses(true),
				),
				// Keep mermaid fences as plain <pre><code class="language-mermaid">
				// instead of wrapping them in chroma markup.
				highlighting.WithExcludeLanguages("mermaid"),
			),
		),
	)

	return &Renderer{
		parser: p,
		html:   r,
	}
}

// Render converts Markdown to HTML. YAML frontmatter (--- ... ---) is stripped
// before rendering so it does not appear as an <hr> in the output.
func (r *Renderer) Render(source []byte) ([]byte, error) {
	_, body, _ := ParseFrontmatter(source)
	var buf bytes.Buffer
	doc := r.parser.Parse(body)
	if err := r.html.Render(&buf, body, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
