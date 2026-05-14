import { useCallback, useEffect, useState } from "react";

type StorageLike = Pick<Storage, "getItem" | "setItem">;

const resolveStorage = (override?: StorageLike): StorageLike | null => {
  if (override) return override;
  if (typeof window === "undefined") return null;
  return window.localStorage;
};

export function useLocalStorageBoolean(
  key: string,
  defaultValue: boolean,
  storage?: StorageLike,
): [boolean, (next: boolean | ((prev: boolean) => boolean)) => void] {
  const [value, setValue] = useState<boolean>(() => {
    const s = resolveStorage(storage);
    if (!s) return defaultValue;
    const raw = s.getItem(key);
    if (raw === "true") return true;
    if (raw === "false") return false;
    return defaultValue;
  });

  useEffect(() => {
    const s = resolveStorage(storage);
    if (!s) return;
    const target = value ? "true" : "false";
    if (s.getItem(key) !== target) {
      s.setItem(key, target);
    }
  }, [key, value, storage]);

  const update = useCallback((next: boolean | ((prev: boolean) => boolean)) => {
    setValue((prev) =>
      typeof next === "function"
        ? (next as (prev: boolean) => boolean)(prev)
        : next,
    );
  }, []);

  return [value, update];
}
