package markdown

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// NoteMeta holds the structured frontmatter fields GophDrive recognizes.
type NoteMeta struct {
	Tags    []string // frontmatter tags only (inline #tags are ExtractTags' job)
	Aliases []string // alternate titles for wikilink resolution and search
	Type    string   // note type, e.g. "decision", "howto"
	Status  string   // workflow status, e.g. "active", "superseded"
}

// ParseNoteMeta extracts recognized frontmatter fields from source. Returns
// the document body with frontmatter removed, and whether frontmatter was
// present. Accepts both YAML sequence and CSV string for "tags" and
// "aliases".
func ParseNoteMeta(source []byte) (meta NoteMeta, body []byte, ok bool) {
	m := frontmatterRe.FindSubmatch(source)
	if m == nil {
		return NoteMeta{}, source, false
	}
	body = source[len(m[0]):]
	var fm struct {
		Tags    interface{} `yaml:"tags"`
		Aliases interface{} `yaml:"aliases"`
		Type    string      `yaml:"type"`
		Status  string      `yaml:"status"`
	}
	if err := yaml.Unmarshal(m[1], &fm); err != nil {
		return NoteMeta{}, body, true
	}
	meta = NoteMeta{
		Tags:    normalizeTags(fm.Tags),
		Aliases: normalizeTags(fm.Aliases),
		Type:    strings.TrimSpace(fm.Type),
		Status:  strings.TrimSpace(fm.Status),
	}
	return meta, body, true
}
