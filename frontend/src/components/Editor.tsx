import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { oneDark } from "@codemirror/theme-one-dark";
import { EditorView } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import type { TransactionSpec } from "@codemirror/state";

import { useTheme } from "@/components/ThemeProvider";

export type EditorHandle = {
  runCommand(fn: (state: EditorState) => TransactionSpec): void;
  focus(): void;
};

interface EditorProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  readOnly?: boolean;
}

export const Editor = forwardRef<EditorHandle, EditorProps>(function Editor(
  { value, onChange, className, readOnly = false },
  ref,
) {
  const { resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  const viewRef = useRef<EditorView | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useImperativeHandle(
    ref,
    () => ({
      runCommand(fn) {
        if (!viewRef.current) return;
        viewRef.current.dispatch(
          viewRef.current.state.update(fn(viewRef.current.state)),
        );
      },
      focus() {
        viewRef.current?.focus();
      },
    }),
    [],
  );

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
        onCreateEditor={(v) => {
          viewRef.current = v;
        }}
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
});
Editor.displayName = "Editor";
