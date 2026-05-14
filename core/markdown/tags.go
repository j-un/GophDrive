package markdown

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	frontmatterRe = regexp.MustCompile(`(?s)^---\r?\n(.*?)\n---\r?\n?`)
	codeFenceRe   = regexp.MustCompile("(?s)```[^`]*```")
	inlineCodeRe  = regexp.MustCompile("`[^`\n]+`")
	headingRe     = regexp.MustCompile(`^#{1,6}\s`)
	// inlineTagRe matches #tag preceded by line-start or whitespace/punctuation.
	// First char of tag must be a Unicode letter or underscore (excludes #1, #123).
	inlineTagRe = regexp.MustCompile(`(?:^|[\s　、。，．！？「」『』【】（）\[\]])#([\p{L}_][\p{L}\p{N}_\-/]*)`)
)

// ParseFrontmatter extracts YAML frontmatter from the start of source.
// Returns the tags list, the document body with frontmatter removed, and whether
// frontmatter was present. Accepts both YAML sequence and CSV string for the
// "tags" key.
func ParseFrontmatter(source []byte) (tags []string, body []byte, ok bool) {
	m := frontmatterRe.FindSubmatch(source)
	if m == nil {
		return nil, source, false
	}
	body = source[len(m[0]):]
	var fm struct {
		Tags interface{} `yaml:"tags"`
	}
	if err := yaml.Unmarshal(m[1], &fm); err != nil || fm.Tags == nil {
		return nil, body, true
	}
	return normalizeTags(fm.Tags), body, true
}

// normalizeTags converts a YAML tags value (list or comma-separated string) to []string.
func normalizeTags(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		var out []string
		for _, s := range strings.Split(t, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ExtractTags returns all unique tags from source, combining frontmatter tags
// and inline #tag syntax. Result is deduplicated and sorted.
func ExtractTags(source []byte) []string {
	fmTags, body, _ := ParseFrontmatter(source)
	inTags := extractInlineTags(body)

	seen := make(map[string]bool, len(fmTags)+len(inTags))
	all := make([]string, 0, len(fmTags)+len(inTags))
	for _, t := range append(fmTags, inTags...) {
		if t != "" && !seen[t] {
			seen[t] = true
			all = append(all, t)
		}
	}
	sort.Strings(all)
	return all
}

// extractInlineTags finds #tag patterns in body text, excluding code fences,
// inline code spans, and ATX headings.
func extractInlineTags(body []byte) []string {
	body = codeFenceRe.ReplaceAll(body, nil)
	body = inlineCodeRe.ReplaceAll(body, nil)

	var tags []string
	for _, line := range bytes.Split(body, []byte("\n")) {
		if headingRe.Match(line) {
			continue
		}
		for _, m := range inlineTagRe.FindAllSubmatch(line, -1) {
			tag := strings.TrimRight(string(m[1]), "-_/")
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}
