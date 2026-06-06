"use client";

import React, { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useWasm } from "@/hooks/useWasm";
import { linkifyTags } from "@/lib/linkifyTags";
import { linkifyWikilinks, WikiLinkRef } from "@/lib/linkifyWikilinks";
import styles from "./markdown.module.css";

interface PreviewProps {
  markdown: string;
  links?: WikiLinkRef[];
  className?: string;
}

export function Preview({ markdown, links, className }: PreviewProps) {
  const { isReady } = useWasm();
  const [html, setHtml] = useState<string>("");
  const router = useRouter();

  useEffect(() => {
    if (isReady && window.renderMarkdown) {
      try {
        const raw = window.renderMarkdown(markdown);
        const withTags = linkifyTags(raw);
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setHtml(linkifyWikilinks(withTags, links ?? []));
      } catch (e) {
        console.error("Render error", e);
        setHtml('<p style="color:red">Error rendering markdown</p>');
      }
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
