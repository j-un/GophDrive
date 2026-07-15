import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
} from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorView } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import type { TransactionSpec } from "@codemirror/state";

import { quietEditorExtensions } from "@/lib/editorTheme";

export type SelectionInfo = {
  from: number;
  to: number;
  empty: boolean;
};

export type EditorHandle = {
  runCommand(fn: (state: EditorState) => TransactionSpec): void;
  focus(): void;
  getView(): EditorView | null;
};

interface EditorProps {
  value: string;
  onChange: (value: string) => void;
  className?: string;
  readOnly?: boolean;
  onSelectionChange?: (sel: SelectionInfo) => void;
}

export const Editor = forwardRef<EditorHandle, EditorProps>(function Editor(
  { value, onChange, className, readOnly = false, onSelectionChange },
  ref,
) {
  const viewRef = useRef<EditorView | null>(null);

  // Hold `onSelectionChange` in a ref so the CodeMirror update listener always
  // calls the latest callback without needing to rebuild extensions.
  const selectionCbRef = useRef<EditorProps["onSelectionChange"]>(undefined);
  selectionCbRef.current = onSelectionChange;

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
      getView() {
        return viewRef.current;
      },
    }),
    [],
  );

  const handleChange = useCallback(
    (val: string) => {
      onChange(val);
    },
    [onChange],
  );

  // Extensions are stable across renders — the selection listener reads the
  // callback from a ref so we don't churn the extension array.
  const extensions = useMemo(
    () => [
      markdown({ base: markdownLanguage, codeLanguages: languages }),
      EditorView.lineWrapping,
      ...quietEditorExtensions,
      EditorView.updateListener.of((update) => {
        if (!update.selectionSet && !update.docChanged) return;
        const cb = selectionCbRef.current;
        if (!cb) return;
        const main = update.state.selection.main;
        cb({ from: main.from, to: main.to, empty: main.empty });
      }),
    ],
    [],
  );

  return (
    <div
      className={className}
      style={{
        backgroundColor: "var(--background)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <CodeMirror
        value={value}
        height="auto"
        minHeight="100%"
        className="flex-1"
        extensions={extensions}
        onChange={handleChange}
        onCreateEditor={(v) => {
          viewRef.current = v;
        }}
        theme="none"
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
