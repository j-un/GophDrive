import React, { useMemo, useCallback } from "react";
import { useNavigate } from "react-router";
import { useWasm } from "@/hooks/useWasm";
import { linkifyTags } from "@/lib/linkifyTags";
import { linkifyWikilinks, WikiLinkRef } from "@/lib/linkifyWikilinks";
import { sanitizeRenderedMarkdown } from "@/lib/sanitize";
import styles from "./markdown.module.css";

interface PreviewProps {
  markdown: string;
  links?: WikiLinkRef[];
  className?: string;
  // Whether this Preview is currently visible. When false, the Wasm
  // render/sanitize pipeline is skipped entirely (the caller may keep this
  // component mounted while hidden — e.g. behind a CSS class — to avoid
  // remount cost). Defaults to true so existing callers render as before.
  // Since active is a useMemo dep, flipping it back to true recomputes html
  // from the then-current markdown in the same render, before anything
  // paints — no stale content is ever shown once visible again.
  active?: boolean;
}

export function Preview({
  markdown,
  links,
  className,
  active = true,
}: PreviewProps) {
  const { isReady } = useWasm();
  const navigate = useNavigate();

  // isReady gates window access — safe in SSR/static-generation contexts where
  // isReady stays false and window is undefined.
  // TODO: client-side sanitize is the sole XSS defense today; remove
  // html.WithUnsafe() from core/markdown/renderer.go for defense in depth.
  const html = useMemo(() => {
    if (!active || !isReady || !window.renderMarkdown) return "";
    try {
      const raw = window.renderMarkdown(markdown);
      const withTags = linkifyTags(raw);
      const linked = linkifyWikilinks(withTags, links ?? []);
      return sanitizeRenderedMarkdown(linked);
    } catch (e) {
      console.error("Render error", e);
      return '<p style="color:red">Error rendering markdown</p>';
    }
  }, [markdown, isReady, links, active]);

  const handleClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      const target = e.target as HTMLElement;
      const anchor = target.closest(
        "a.wikilink[data-note-id]",
      ) as HTMLAnchorElement | null;
      if (anchor) {
        e.preventDefault();
        const noteId = anchor.getAttribute("data-note-id");
        if (noteId) navigate(`/note/?id=${noteId}`);
      }
    },
    [navigate],
  );

  if (!isReady) {
    return (
      <div
        className={`p-4 opacity-50 flex items-center justify-center h-full ${className || ""}`}
      >
        <p>Loading renderer...</p>
      </div>
    );
  }

  return (
    <div
      className={`${styles.preview} ${className || ""}`}
      dangerouslySetInnerHTML={{ __html: html }}
      onClick={handleClick}
    />
  );
}
