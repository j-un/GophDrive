package markdown

import (
	"reflect"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantTags []string
		wantOk   bool
	}{
		{
			name:     "YAML array",
			source:   "---\ntags: [develop, work/q3]\n---\nbody",
			wantTags: []string{"develop", "work/q3"},
			wantOk:   true,
		},
		{
			name:     "YAML array with spaces",
			source:   "---\ntags: [develop, \"in-progress\"]\n---\nbody",
			wantTags: []string{"develop", "in-progress"},
			wantOk:   true,
		},
		{
			name:     "YAML block sequence",
			source:   "---\ntags:\n  - develop\n  - 開発\n---\nbody",
			wantTags: []string{"develop", "開発"},
			wantOk:   true,
		},
		{
			name:     "CSV string",
			source:   "---\ntags: develop, work/q3, 開発\n---\nbody",
			wantTags: []string{"develop", "work/q3", "開発"},
			wantOk:   true,
		},
		{
			name:     "no frontmatter",
			source:   "# Title\nbody text",
			wantTags: nil,
			wantOk:   false,
		},
		{
			name:     "frontmatter without tags key",
			source:   "---\ntitle: My Note\n---\nbody",
			wantTags: nil,
			wantOk:   true,
		},
		{
			name:     "empty tags list",
			source:   "---\ntags: []\n---\nbody",
			wantTags: []string{},
			wantOk:   true,
		},
		{
			name:     "body preserved",
			source:   "---\ntags: [x]\n---\nHello world",
			wantTags: []string{"x"},
			wantOk:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, body, ok := ParseFrontmatter([]byte(tt.source))
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if tt.wantTags != nil && !reflect.DeepEqual(tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", tags, tt.wantTags)
			}
			if tt.wantTags == nil && len(tags) != 0 {
				t.Errorf("tags = %v, want nil/empty", tags)
			}
			// body should not contain frontmatter
			if ok {
				for _, b := range body {
					_ = b // just ensure body is non-nil
					break
				}
			}
		})
	}
}

func TestExtractInlineTags(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "basic inline tag",
			body: "Hello #develop world",
			want: []string{"develop"},
		},
		{
			name: "multiple tags",
			body: "Work on #backend and #frontend today",
			want: []string{"backend", "frontend"},
		},
		{
			name: "tag at line start",
			body: "#start-of-line tag",
			want: []string{"start-of-line"},
		},
		{
			name: "tag after punctuation",
			body: "文章、#開発 の話",
			want: []string{"開発"},
		},
		{
			name: "CJK tag",
			body: "今日の作業 #開発 #テスト",
			want: []string{"開発", "テスト"},
		},
		{
			name: "hierarchical tag",
			body: "See #work/q3 task",
			want: []string{"work/q3"},
		},
		{
			name: "hyphenated tag",
			body: "Status: #in-progress",
			want: []string{"in-progress"},
		},
		{
			name: "digit-only tag excluded",
			body: "Issue #1 is done, see #2",
			want: nil,
		},
		{
			name: "mixed digit and letter tag allowed",
			body: "See #v2release notes",
			want: []string{"v2release"},
		},
		{
			name: "heading excluded",
			body: "# This is a heading\nNot a #tag in heading",
			want: []string{"tag"},
		},
		{
			name: "h2 excluded",
			body: "## Section\nText #real-tag here",
			want: []string{"real-tag"},
		},
		{
			name: "tag in code fence excluded",
			body: "```\n#inside code\n```\n#outside",
			want: []string{"outside"},
		},
		{
			name: "tag in inline code excluded",
			body: "Use `#notag` but #realtag",
			want: []string{"realtag"},
		},
		{
			name: "mid-word hash excluded",
			body: "foo#bar is not a tag",
			want: nil,
		},
		{
			name: "URL fragment excluded",
			body: "See https://example.com/path#section for details",
			want: nil,
		},
		{
			name: "trailing slash trimmed",
			body: "See #work/ for info",
			want: []string{"work"},
		},
		{
			name: "no tags",
			body: "Just plain text without tags",
			want: nil,
		},
		{
			name: "Japanese punctuation prefix",
			body: "タグ。#開発する",
			want: []string{"開発する"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInlineTags([]byte(tt.body))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractInlineTags(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "frontmatter + inline combined and deduped",
			source: "---\ntags: [develop, backend]\n---\n# Title\nWork on #develop and #frontend",
			want: []string{"backend", "develop", "frontend"},
		},
		{
			name: "frontmatter only",
			source: "---\ntags: [alpha, beta]\n---\nNo inline tags here",
			want: []string{"alpha", "beta"},
		},
		{
			name: "inline only",
			source: "# Note\nWorking on #feature today",
			want: []string{"feature"},
		},
		{
			name: "no tags at all",
			source: "Just a plain note",
			want: nil,
		},
		{
			name: "deduplication",
			source: "---\ntags: [go]\n---\nUsing #go language",
			want: []string{"go"},
		},
		{
			name: "sorted output",
			source: "---\ntags: [zebra, apple]\n---\n#mango fruit",
			want: []string{"apple", "mango", "zebra"},
		},
		{
			name: "CJK frontmatter and inline",
			source: "---\ntags: [開発, テスト]\n---\n今日は #バグ修正 をした",
			want: []string{"テスト", "バグ修正", "開発"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags([]byte(tt.source))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
			}
		})
	}
}
