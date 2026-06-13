/**
 * Builds a clipboard-ready string for the Share button.
 * Returns `[title](url)` when title is non-empty (with Markdown-unsafe
 * characters escaped), or a bare URL when the title is empty.
 */
export function buildShareLink(title: string, url: string): string {
  const trimmed = title.trim();
  return trimmed ? `[${trimmed.replace(/[\\[\]]/g, "\\$&")}](${url})` : url;
}
