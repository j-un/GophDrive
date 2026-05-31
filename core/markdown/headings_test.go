package markdown

import (
	"reflect"
	"testing"
)

func TestExtractHeadings_Basic(t *testing.T) {
	src := []byte("## Background\ntext\n## Decision\nmore\n### Sub-section\ndeep\n")
	got := ExtractHeadings(src)
	want := []string{"Background", "Decision", "Sub-section"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_SkipsCodeFence(t *testing.T) {
	src := []byte("## Real\n```\n## Not a heading\n```\n## Also Real\n")
	got := ExtractHeadings(src)
	want := []string{"Real", "Also Real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_SkipsFrontmatter(t *testing.T) {
	src := []byte("---\ntags: [foo]\n---\n## Actual Heading\n")
	got := ExtractHeadings(src)
	want := []string{"Actual Heading"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_MixedLevels(t *testing.T) {
	src := []byte("# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6\n")
	got := ExtractHeadings(src)
	want := []string{"H1", "H2", "H3", "H4", "H5", "H6"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_DuplicatesPreserved(t *testing.T) {
	src := []byte("## Background\ntext\n## Background\nmore\n")
	got := ExtractHeadings(src)
	want := []string{"Background", "Background"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_Empty(t *testing.T) {
	got := ExtractHeadings([]byte("no headings here"))
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestExtractHeadings_TildeFence(t *testing.T) {
	src := []byte("## Real\n~~~\n## Inside tilde fence\n~~~\n## Also Real\n")
	got := ExtractHeadings(src)
	want := []string{"Real", "Also Real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_BlockquoteSkipped(t *testing.T) {
	src := []byte("## Real\n> ## Quoted heading\n## Also Real\n")
	got := ExtractHeadings(src)
	want := []string{"Real", "Also Real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_IndentedCodeSkipped(t *testing.T) {
	src := []byte("## Real\n    ## Indented heading (code block)\n## Also Real\n")
	got := ExtractHeadings(src)
	want := []string{"Real", "Also Real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractHeadings_CRLF(t *testing.T) {
	src := []byte("## Background\r\ntext\r\n## Decision\r\n")
	got := ExtractHeadings(src)
	want := []string{"Background", "Decision"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CRLF: got %v, want %v", got, want)
	}
}
