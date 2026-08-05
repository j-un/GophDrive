import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";
import CodeMirror, { ExternalChange } from "@uiw/react-codemirror";
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
  /**
   * The document text. CodeMirror owns the live document and this prop only
   * mirrors it, so the editor writes `value` back into the document in exactly
   * one case: when it is a string the editor never emitted — a note load, or
   * the conflict dialog's "Discard Local Changes". Pushing text in that way is
   * the supported way to drive the editor from outside, and it replaces the
   * whole document.
   */
  value: string;
  /**
   * Called with the full document text after every edit.
   *
   * The caller must route the emitted string back into `value` verbatim — no
   * trimming, no newline or whitespace normalization, no filtering. The editor
   * recognizes its own output by exact string match to tell a lagging echo
   * apart from an external change; a transformed string fails that match, so it
   * is taken for an external change and overwrites the document, discarding
   * everything typed since. If every echo comes back altered, that overwrite
   * repeats on each keystroke.
   */
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

  // CodeMirror needs one extra commit to attach its container, so the view does
  // not exist on the first render. `viewRef` backs the imperative handle; this
  // state copy is what lets the sync effect below re-run the moment the view
  // shows up.
  const [view, setView] = useState<EditorView | null>(null);

  // Hold `onSelectionChange` in a ref so the CodeMirror update listener always
  // calls the latest callback without needing to rebuild extensions.
  const selectionCbRef = useRef<EditorProps["onSelectionChange"]>(undefined);
  selectionCbRef.current = onSelectionChange;

  // Frozen at mount — the only part of `value` <CodeMirror> is still allowed to
  // see. See the note above the element.
  const [initialValue] = useState(value);

  // Values this editor pushed out through `onChange`, oldest first. CodeMirror
  // owns its document and updates it synchronously on every keystroke, so the
  // `value` prop is a mirror that trails by at least one commit: a `value`
  // listed here is our own echo coming back and may already be outdated, so it
  // must never be written to the document. Anything else is a genuine external
  // change (note load, conflict resolution) and does get written.
  const echoRef = useRef<string[]>([]);

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
      echoRef.current.push(val);
      onChange(val);
    },
    [onChange],
  );

  // Writes `next` into the document unless it is one of our own echoes, in
  // which case the document stays authoritative. A matched echo is drained
  // along with everything older, so the list never outgrows the commits still
  // in flight. The write is tagged `ExternalChange` — the same annotation the
  // library uses for its own controlled-value writes — which keeps it out of
  // `onChange`: the caller handed us this text, so sending it straight back
  // would only be a round trip through the parent's state.
  const syncDoc = useCallback((target: EditorView, next: string) => {
    const idx = echoRef.current.indexOf(next);
    if (idx >= 0) {
      echoRef.current.splice(0, idx + 1);
      return;
    }
    echoRef.current.length = 0;
    if (next === target.state.doc.toString()) return;
    target.dispatch({
      changes: { from: 0, to: target.state.doc.length, insert: next },
      annotations: [ExternalChange.of(true)],
    });
  }, []);

  useEffect(() => {
    if (!view) return;
    syncDoc(view, value);
  }, [value, view, syncDoc]);

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
      {/*
        `value` is pinned to the mount-time text on purpose. @uiw/react-codemirror
        (4.25.10, unfixed upstream) re-runs its controlled-value effect on every
        prop change and, while the user is typing, parks the value it captured
        behind a 200ms latch that it never disarms once the prop catches up — so
        a commit that trailed the document gets replayed later and overwrites
        every character typed since. Pinning the prop keeps that effect's
        dependencies constant: it fires once, when the view is created and the
        document already equals the prop, and never again. `syncDoc` is then the
        only writer, and it refuses to write anything the editor itself emitted.
        (Omitting the prop is not an option — the library defaults it to "".)
      */}
      <CodeMirror
        value={initialValue}
        height="auto"
        minHeight="100%"
        className="flex-1"
        extensions={extensions}
        onChange={handleChange}
        onCreateEditor={(v) => {
          viewRef.current = v;
          setView(v);
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
