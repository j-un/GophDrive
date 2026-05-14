// Converts #tag patterns in rendered HTML to clickable links.
// Skips content inside <pre>/<code> blocks. The prefix group (whitespace,
// punctuation, or > for HTML tag close) prevents mid-word # matches.
export function linkifyTags(html: string): string {
  // /mu: m for ^ line-start matching, u for \p{} Unicode property escapes
  return html.replace(
    /(<(?:pre|code)[^>]*>[\s\S]*?<\/(?:pre|code)>)|([\s>　、。，．！？「」『』【】（）\[\]]|^)#([\p{L}_][\p{L}\p{N}_\-/]*)/gmu,
    (match, codeBlock, prefix, tag) => {
      if (codeBlock !== undefined) return codeBlock;
      const cleaned = tag.replace(/[-_/]+$/, "");
      if (!cleaned) return match;
      return `${prefix}<a href="/notes?tag=${encodeURIComponent(cleaned)}" class="tag-link">#${cleaned}</a>`;
    },
  );
}
