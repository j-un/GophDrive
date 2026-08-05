import { useEffect, useRef, useState } from "react";

export function useAutoSave(
  value: string,
  saveFunction: (val: string) => Promise<void>,
  delay: number = 2000,
  enabled: boolean = true,
) {
  const [isSaving, setIsSaving] = useState(false);
  const [lastSavedValue, setLastSavedValue] = useState<string | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Mirror `saveFunction` in a ref so the debounce timer always calls the
  // latest version without needing `saveFunction` in the effect deps below —
  // otherwise a reference change (e.g. the caller's save callback picking up
  // a fresh etag after a prior save) would reset the pending debounce timer.
  const saveFunctionRef = useRef(saveFunction);
  useEffect(() => {
    saveFunctionRef.current = saveFunction;
  }, [saveFunction]);

  useEffect(() => {
    if (!enabled) return;

    if (lastSavedValue === null) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLastSavedValue(value);
      return;
    }

    if (value === lastSavedValue) return;

    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    timeoutRef.current = setTimeout(async () => {
      setIsSaving(true);
      setError(null);
      try {
        await saveFunctionRef.current(value);
        setLastSavedValue(value);
      } catch (error) {
        const err = Object.assign(
          new Error(),
          typeof error === "object" ? error : { message: String(error) },
        );
        console.error("AutoSave failed", err);
        setError(err);
      } finally {
        setIsSaving(false);
      }
    }, delay);

    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, [value, delay, lastSavedValue, enabled]);

  return {
    isSaving,
    hasUnsavedChanges: lastSavedValue !== null && value !== lastSavedValue,
    error,
  };
}
