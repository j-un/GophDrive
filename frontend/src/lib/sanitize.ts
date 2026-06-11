import DOMPurify from "dompurify";

let hookRegistered = false;

export function sanitizeRenderedMarkdown(html: string): string {
  if (typeof window === "undefined") return "";
  if (!hookRegistered) {
    // Enforce rel="noopener noreferrer" on any target="_blank" link surviving
    // sanitization. The Go renderer uses html.WithUnsafe(), so user-authored
    // <a target="_blank"> would otherwise expose the app to reverse-tabnabbing.
    DOMPurify.addHook("afterSanitizeAttributes", (node) => {
      if (node.tagName === "A" && node.getAttribute("target") === "_blank") {
        node.setAttribute("rel", "noopener noreferrer");
      }
    });
    hookRegistered = true;
  }
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    // data-note-id: wikilink click handler reads this attribute.
    // target: preserved so user-authored target="_blank" links work; the
    //         afterSanitizeAttributes hook above enforces rel="noopener noreferrer".
    ADD_ATTR: ["data-note-id", "target"],
    KEEP_CONTENT: true,
    SAFE_FOR_TEMPLATES: false,
  });
}
