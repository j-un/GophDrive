"use client";

import React, { useEffect, useState } from "react";
import { useWasm } from "@/hooks/useWasm";
import styles from "./markdown.module.css";

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
        const result = window.renderMarkdown(markdown);
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setHtml(result);
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
