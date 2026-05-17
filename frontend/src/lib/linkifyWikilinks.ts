// Converts [[Title]] wiki-link patterns in rendered HTML to clickable links.
// Skips content inside <pre>/<code> blocks.
// Uses the links array (from GET /notes/{id}) for resolved targets and currentTitles.

export interface WikiLinkRef {
  title: string;
  targetId?: string;
  currentTitle?: string;
  resolved: boolean;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function linkifyWikilinks(html: string, links: WikiLinkRef[]): string {
  if (!links || links.length === 0) return html;

  // Build lookup by written title (case-insensitive key matches backend normalizeTitle)
  const byTitle = new Map<string, WikiLinkRef>();
  for (const l of links) {
    byTitle.set(l.title.toLowerCase().trim(), l);
  }

  // Pattern: code/pre blocks OR [[...]] tokens outside them. The tag name is
  // captured and back-referenced (\2) so an opening <pre> only closes on its
  // matching </pre> — a nested <pre><code>…</code></pre> (or any token between
  // </code> and </pre>) is skipped as one unit instead of leaking out.
  const re = /(<(pre|code)[^>]*>[\s\S]*?<\/\2>)|\[\[([^\[\]\n|#]+?)\]\]/g;

  return html.replace(re, (match, codeBlock, _tag, rawTitle) => {
    if (codeBlock !== undefined) return codeBlock;

    const title = rawTitle.trim();
    const ref = byTitle.get(title.toLowerCase().trim());

    if (ref && ref.resolved && ref.targetId) {
      const label = escapeHtml(ref.currentTitle || title);
      return `<a class="wikilink" data-note-id="${escapeHtml(ref.targetId)}" href="/note?id=${escapeHtml(ref.targetId)}">${label}</a>`;
    }

    const label = escapeHtml(title);
    return `<a class="wikilink wikilink-unresolved" data-note-title="${escapeHtml(title)}" href="#">${label}</a>`;
  });
}
