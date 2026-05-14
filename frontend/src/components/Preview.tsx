"use client";

import React, { useEffect, useState } from "react";
import { useWasm } from "@/hooks/useWasm";
import styles from "./markdown.module.css";

// Converts #tag patterns in rendered HTML to clickable links.
// Skips content inside <pre>/<code> blocks. The regex captures a mandatory
// prefix (whitespace/punctuation or line start) to avoid mid-word # matches.
function linkifyTags(html: string): string {
  // /mu: m for ^ line-start matching, u for \p{} Unicode property escapes
  return html.replace(
    /(<(?:pre|code)[^>]*>[\s\S]*?<\/(?:pre|code)>)|([\s　、。，．！？「」『』【】（）\[\]]|^)#([\p{L}_][\p{L}\p{N}_\-/]*)/gmu,
    (match, codeBlock, prefix, tag) => {
      if (codeBlock !== undefined) return codeBlock;
      const cleaned = tag.replace(/[-_/]+$/, "");
      if (!cleaned) return match;
      return `${prefix}<a href="/notes?tag=${encodeURIComponent(cleaned)}" class="tag-link">#${cleaned}</a>`;
    },
  );
}

interface PreviewProps {
  markdown: string;
  className?: string;
}

export function Preview({ markdown, className }: PreviewProps) {
  const { isReady } = useWasm();
  const [html, setHtml] = useState<string>("");

  useEffect(() => {
    if (isReady && window.renderMarkdown) {
      try {
        const raw = window.renderMarkdown(markdown);
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setHtml(linkifyTags(raw));
      } catch (e) {
        console.error("Render error", e);
        setHtml('<p style="color:red">Error rendering markdown</p>');
      }
    }
  }, [markdown, isReady]);

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
    />
  );
}
