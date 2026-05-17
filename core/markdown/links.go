package markdown

import (
	"regexp"
	"sort"
	"strings"
)

var wikiLinkRe = regexp.MustCompile(`\[\[\s*([^\[\]\n|#]+?)\s*\]\]`)

// ExtractWikiLinks returns all unique [[wiki-link]] titles from source.
// Titles inside code fences and inline code are excluded. Frontmatter is
// stripped before scanning. Result is deduplicated and sorted.
func ExtractWikiLinks(source []byte) []string {
	_, body, _ := ParseFrontmatter(source)

	body = codeFenceRe.ReplaceAll(body, nil)
	body = inlineCodeRe.ReplaceAll(body, nil)

	seen := make(map[string]bool)
	var titles []string
	for _, m := range wikiLinkRe.FindAllSubmatch(body, -1) {
		t := strings.TrimSpace(string(m[1]))
		if t != "" && !seen[t] {
			seen[t] = true
			titles = append(titles, t)
		}
	}
	sort.Strings(titles)
	return titles
}
