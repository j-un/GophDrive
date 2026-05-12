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

  useEffect(() => {
    if (!enabled) return;

    if (lastSavedValue === null) {
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
        await saveFunction(value);
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
  }, [value, saveFunction, delay, lastSavedValue, enabled]);

  return {
    isSaving,
    hasUnsavedChanges: lastSavedValue !== null && value !== lastSavedValue,
    error,
  };
}
