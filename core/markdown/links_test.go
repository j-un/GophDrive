package markdown

import (
	"reflect"
	"testing"
)

func TestExtractWikiLinks(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "none",
			source: "no links here",
			want:   nil,
		},
		{
			name:   "single",
			source: "see [[Auth Design]]",
			want:   []string{"Auth Design"},
		},
		{
			name:   "multiple",
			source: "see [[Auth Design]] and [[API Guide]]",
			want:   []string{"API Guide", "Auth Design"},
		},
		{
			name:   "deduplicated and sorted",
			source: "[[B]] and [[A]] and [[B]]",
			want:   []string{"A", "B"},
		},
		{
			name:   "trims whitespace",
			source: "[[  Note Name  ]]",
			want:   []string{"Note Name"},
		},
		{
			name:   "frontmatter excluded",
			source: "---\ntitle: test\n---\n[[FrontmatterLink]]\n[[BodyLink]]",
			want:   []string{"BodyLink", "FrontmatterLink"},
		},
		{
			name:   "code fence excluded",
			source: "```\n[[InFence]]\n```\n[[Outside]]",
			want:   []string{"Outside"},
		},
		{
			name:   "inline code excluded",
			source: "use `[[InCode]]` and [[Real]]",
			want:   []string{"Real"},
		},
		{
			name:   "empty brackets ignored",
			source: "[[]] and [[  ]]",
			want:   nil,
		},
		{
			name:   "newline inside brackets ignored",
			source: "[[line1\nline2]]",
			want:   nil,
		},
		{
			name:   "pipe alias not parsed (v1)",
			source: "[[Title|alias]] and [[Plain]]",
			want:   []string{"Plain"},
		},
		{
			name:   "heading anchor not parsed (v1)",
			source: "[[Title#heading]] and [[Plain]]",
			want:   []string{"Plain"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractWikiLinks([]byte(tc.source))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractWikiLinks(%q) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}
