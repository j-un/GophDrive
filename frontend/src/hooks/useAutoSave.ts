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

  const saveFunctionRef = useRef(saveFunction);
  useEffect(() => {
    saveFunctionRef.current = saveFunction;
  }, [saveFunction]);

  if (enabled && lastSavedValue === null) {
    setLastSavedValue(value);
  }

  useEffect(() => {
    if (!enabled || lastSavedValue === null || value === lastSavedValue) return;

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
