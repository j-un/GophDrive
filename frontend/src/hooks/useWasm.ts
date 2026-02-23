import { useState, useEffect } from "react";
import { initWasm } from "@/lib/wasm-loader";

export function useWasm() {
  const [isReady, setIsReady] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    initWasm()
      .then(() => setIsReady(true))
      .catch((err: unknown) => {
        console.error(err);
        setError(err instanceof Error ? err : new Error(String(err)));
      });
  }, []);

  return { isReady, error };
}
