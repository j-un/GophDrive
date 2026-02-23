import { useEffect, useRef, useState } from "react";

export function useAutoSave(
  value: string,
  saveFunction: (val: string) => Promise<void>,
  delay: number = 2000,
) {
  const [isSaving, setIsSaving] = useState(false);
  const [lastSavedValue, setLastSavedValue] = useState(value);
  const [error, setError] = useState<Error | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
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
  }, [value, saveFunction, delay, lastSavedValue]);

  return { isSaving, hasUnsavedChanges: value !== lastSavedValue, error };
}
