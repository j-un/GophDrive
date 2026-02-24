"use client";

import React from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { oneDark } from "@codemirror/theme-one-dark";
import { EditorView } from "@codemirror/view";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

interface EditorProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  readOnly?: boolean;
}

export function Editor({
  value,
  onChange,
  className,
  readOnly = false,
}: EditorProps) {
  const { resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMounted(true);
  }, []);

  const handleChange = React.useCallback(
    (val: string) => {
      onChange(val);
    },
    [onChange],
  );

  return (
    <div
      className={`rounded-lg border border-[var(--border)] ${className || ""}`}
      style={{
        backgroundColor: "var(--card)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <CodeMirror
        value={value}
        height="auto"
        minHeight="100%"
        className="text-base font-mono flex-1"
        extensions={[
          markdown({ base: markdownLanguage, codeLanguages: languages }),
          EditorView.lineWrapping,
        ]}
        onChange={handleChange}
        theme={mounted && resolvedTheme === "dark" ? oneDark : undefined}
        readOnly={readOnly}
        basicSetup={{
          lineNumbers: false,
          foldGutter: false,
          highlightActiveLine: false,
          autocompletion: false,
        }}
      />
    </div>
  );
}
