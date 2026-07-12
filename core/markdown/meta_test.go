package markdown

import (
	"reflect"
	"testing"
)

func TestParseNoteMeta(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		wantMeta  NoteMeta
		wantOk    bool
		wantBody  string
		checkBody bool
	}{
		{
			name:   "aliases as YAML list",
			source: "---\naliases: [Foo, Bar Baz]\n---\nbody",
			wantMeta: NoteMeta{
				Aliases: []string{"Foo", "Bar Baz"},
			},
			wantOk: true,
		},
		{
			name:   "aliases as CSV string",
			source: "---\naliases: Foo, Bar Baz, 開発\n---\nbody",
			wantMeta: NoteMeta{
				Aliases: []string{"Foo", "Bar Baz", "開発"},
			},
			wantOk: true,
		},
		{
			name:   "only type present",
			source: "---\ntype: decision\n---\nbody",
			wantMeta: NoteMeta{
				Type: "decision",
			},
			wantOk: true,
		},
		{
			name:      "missing fields",
			source:    "---\ntitle: My Note\n---\nbody",
			wantMeta:  NoteMeta{},
			wantOk:    true,
			wantBody:  "body",
			checkBody: true,
		},
		{
			name:      "malformed YAML",
			source:    "---\naliases: [unclosed\n---\nbody",
			wantMeta:  NoteMeta{},
			wantOk:    true,
			wantBody:  "body",
			checkBody: true,
		},
		{
			name:   "CRLF line endings in frontmatter",
			source: "---\r\ntype: howto\r\nstatus: active\r\n---\r\nbody",
			wantMeta: NoteMeta{
				Type:   "howto",
				Status: "active",
			},
			wantOk:    true,
			wantBody:  "body",
			checkBody: true,
		},
		{
			name:      "no frontmatter at all",
			source:    "# Title\nbody text",
			wantMeta:  NoteMeta{},
			wantOk:    false,
			wantBody:  "# Title\nbody text",
			checkBody: true,
		},
		{
			name:   "frontmatter tags parsed alongside aliases",
			source: "---\ntags: [develop, work/q3]\naliases: [Old Name]\ntype: decision\nstatus: active\n---\nbody",
			wantMeta: NoteMeta{
				Tags:    []string{"develop", "work/q3"},
				Aliases: []string{"Old Name"},
				Type:    "decision",
				Status:  "active",
			},
			wantOk: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body, ok := ParseNoteMeta([]byte(tt.source))
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if !reflect.DeepEqual(meta, tt.wantMeta) {
				t.Errorf("meta = %+v, want %+v", meta, tt.wantMeta)
			}
			if tt.checkBody && string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", string(body), tt.wantBody)
			}
		})
	}
}
