/**
 * Parses a freeform search query, extracting `tag:xxx` tokens as tag filters.
 * The remaining tokens are joined as the full-text query.
 * A bare `tag:` with no value is silently dropped (not added to text or tags).
 *
 * Examples:
 *   "tag:test"            → { text: "",        tags: ["test"] }
 *   "tag:test frontend"   → { text: "frontend", tags: ["test"] }
 *   "tag:a tag:b hello"   → { text: "hello",   tags: ["a", "b"] }
 *   "just text"           → { text: "just text", tags: [] }
 *   "tag:"                → { text: "",         tags: [] }
 */
export function parseSearchQuery(query: string): {
  text: string;
  tags: string[];
} {
  const tags: string[] = [];
  const textParts: string[] = [];

  for (const token of query.split(/\s+/).filter(Boolean)) {
    if (token.startsWith("tag:")) {
      const value = token.slice(4);
      if (value) tags.push(value);
      // bare "tag:" with no value is silently dropped
    } else {
      textParts.push(token);
    }
  }

  return { text: textParts.join(" "), tags };
}

/**
 * Merges URL-based tag filters with tags extracted from a freeform query string.
 * Returns the full-text part and a deduplicated tag list ready for searchFiles().
 * Returns undefined for tags when the merged list is empty (signals "no tag filter").
 */
export function buildSearchRequest(
  rawQuery: string | undefined,
  urlTags: string[] | undefined,
): { text: string; tags: string[] | undefined } {
  const { text, tags: queryTags } = parseSearchQuery(rawQuery ?? "");
  const merged = [...(urlTags ?? []), ...queryTags];
  // Dedupe while preserving first-occurrence order
  const seen = new Set<string>();
  const tags = merged.filter((t) => (seen.has(t) ? false : seen.add(t)));
  return { text, tags: tags.length > 0 ? tags : undefined };
}
