package markdown

import (
	"strings"
	"testing"
)

func TestRenderer_Render(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic Markdown",
			input:    "# Hello",
			expected: "<h1 id=\"hello\">Hello</h1>\n",
		},
		{
			name:     "GFM Table",
			input:    "| A | B |\n|---|---|\n| 1 | 2 |",
			expected: "<table>",
		},
		{
			name:     "GFM Task List",
			input:    "- [ ] Task 1\n- [x] Task 2",
			expected: "<input disabled=\"\" type=\"checkbox\"",
		},
		{
			name:     "Mermaid Block",
			input:    "```mermaid\ngraph TD;\n    A-->B;\n```",
			expected: "<pre><code class=\"language-mermaid\">graph TD;\n    A--&gt;B;\n</code></pre>\n",
		},
		{
			name:     "Empty Input",
			input:    "",
			expected: "",
		},
		{
			name:     "GFM Strikethrough",
			input:    "~~deleted~~",
			expected: "<del>deleted</del>",
		},
		{
			name:     "GFM Autolink",
			input:    "Visit https://example.com for more",
			expected: "<a href=\"https://example.com\"",
		},
		{
			name:     "Heading ID auto-generation",
			input:    "## My Section",
			expected: "id=\"my-section\"",
		},
		{
			name:     "Raw HTML passthrough",
			input:    "<div class=\"custom\">raw html</div>",
			expected: "<div class=\"custom\">raw html</div>",
		},
	}

	renderer := NewRenderer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := renderer.Render([]byte(tt.input))
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			got := string(output)
			if !strings.Contains(got, tt.expected) {
				t.Errorf("Render() = %v, want substring %v", got, tt.expected)
			}
		})
	}
}
