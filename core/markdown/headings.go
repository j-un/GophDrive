package markdown

import (
	"bytes"
	"regexp"
	"strings"
)

var headingLineRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// tildeOrBacktickFenceRe matches the opening line of a ~~~ or ``` code fence.
var tildeOrBacktickFenceRe = regexp.MustCompile("^(`{3,}|~{3,})")

// ExtractHeadings returns the plain-text heading texts from source in document
// order, with leading # characters stripped. Only ATX headings (# prefix) are
// extracted; setext headings (=== / --- underlines) are not supported.
// Headings inside code fences (``` or ~~~) and YAML frontmatter are excluded.
// Blockquote lines (starting with >) and lines indented by 4+ spaces are also
// excluded, consistent with CommonMark indented code blocks.
// Duplicate headings are preserved.
func ExtractHeadings(source []byte) []string {
	_, body, _ := ParseFrontmatter(source)

	var headings []string
	inFence := false
	var fenceDelim []byte // the opening delimiter (e.g. "```" or "~~~")

	for _, rawLine := range bytes.Split(body, []byte("\n")) {
		// Strip trailing CR for CRLF sources.
		line := bytes.TrimRight(rawLine, "\r")

		// Track code fence state (``` and ~~~).
		if m := tildeOrBacktickFenceRe.Find(line); m != nil {
			if !inFence {
				inFence = true
				fenceDelim = m
			} else if bytes.HasPrefix(line, fenceDelim) {
				inFence = false
				fenceDelim = nil
			}
			continue
		}
		if inFence {
			continue
		}

		// Skip blockquote lines.
		if bytes.HasPrefix(line, []byte(">")) {
			continue
		}

		// Skip lines indented by 4+ spaces (CommonMark indented code block).
		if bytes.HasPrefix(line, []byte("    ")) || bytes.HasPrefix(line, []byte("\t")) {
			continue
		}

		m := headingLineRe.FindSubmatch(line)
		if m == nil {
			continue
		}
		text := strings.TrimSpace(string(m[2]))
		if text != "" {
			headings = append(headings, text)
		}
	}
	return headings
}
