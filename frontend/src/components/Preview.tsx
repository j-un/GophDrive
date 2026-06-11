"use client";

import React, { useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useWasm } from "@/hooks/useWasm";
import { linkifyTags } from "@/lib/linkifyTags";
import { linkifyWikilinks, WikiLinkRef } from "@/lib/linkifyWikilinks";
import { sanitizeRenderedMarkdown } from "@/lib/sanitize";
import styles from "./markdown.module.css";

interface PreviewProps {
  markdown: string;
  links?: WikiLinkRef[];
  className?: string;
}

export function Preview({ markdown, links, className }: PreviewProps) {
  const { isReady } = useWasm();
  const router = useRouter();

  // isReady gates window access — safe in SSR/static-generation contexts where
  // isReady stays false and window is undefined.
  // TODO: client-side sanitize is the sole XSS defense today; remove
  // html.WithUnsafe() from core/markdown/renderer.go for defense in depth.
  const html = useMemo(() => {
    if (!isReady || !window.renderMarkdown) return "";
    try {
      const raw = window.renderMarkdown(markdown);
      const withTags = linkifyTags(raw);
      const linked = linkifyWikilinks(withTags, links ?? []);
      return sanitizeRenderedMarkdown(linked);
    } catch (e) {
      console.error("Render error", e);
      return '<p style="color:red">Error rendering markdown</p>';
    }
  }, [markdown, isReady, links]);

  const handleClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      const target = e.target as HTMLElement;
      const anchor = target.closest(
        "a.wikilink[data-note-id]",
      ) as HTMLAnchorElement | null;
      if (anchor) {
        e.preventDefault();
        const noteId = anchor.getAttribute("data-note-id");
        if (noteId) router.push(`/note/?id=${noteId}`);
      }
    },
    [router],
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
