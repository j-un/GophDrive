import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";

export function useHeartbeat(fileId: string, isActive: boolean) {
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!fileId || !isActive) return;

    const heartbeat = async () => {
      try {
        const res = await apiFetch(`/sessions/${fileId}/heartbeat`, {
          method: "POST",
        });
        if (!res.ok) {
          setError(`Heartbeat failed: ${res.status}`);
        } else {
          setError(null);
        }
      } catch (error) {
        const err = error as Error;
        setError(err.message || String(error));
      }
    };

    const interval = setInterval(heartbeat, 60000);
    return () => clearInterval(interval);
  }, [fileId, isActive]);

  return { error };
}
