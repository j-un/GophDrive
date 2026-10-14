// Converts #tag patterns in rendered HTML to clickable links.
export function linkifyTags(html: string): string {
  return html.replace(
    /(<(?:pre|code)[^>]*>[\s\S]*?<\/(?:pre|code)>)|([\s>\u3000、。，．！？「」『』【】（）[\]]|^)#([\p{L}_][\p{L}\p{N}_\-/]*)/gmu,
    (match, codeBlock, prefix, tag) => {
      if (codeBlock !== undefined) return codeBlock;
      const cleaned = tag.replace(/[-_/]+$/, "");
      if (!cleaned) return match;
      return `${prefix}<a href="/drive/?tag=${encodeURIComponent(cleaned)}" class="tag-link">#${cleaned}</a>`;
    },
  );
}
