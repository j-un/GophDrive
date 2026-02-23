"use client";

import React from "react";
import { Lock } from "lucide-react";

interface LockBannerProps {
  userId: string;
  expiresAt: number; // Unix timestamp
}

export function LockBanner({ userId, expiresAt }: LockBannerProps) {
  const [timeLeft, setTimeLeft] = React.useState<number>(0);

  React.useEffect(() => {
    const updateTime = () => {
      const remaining = Math.max(0, expiresAt - Math.floor(Date.now() / 1000));
      setTimeLeft(remaining);
    };
    updateTime();
    const interval = setInterval(updateTime, 1000);
    return () => clearInterval(interval);
  }, [expiresAt]);

  if (timeLeft <= 0) return null;

  return (
    <div
      className="bg-amber-500/10 border-b border-amber-500/20 px-4 py-2 flex items-center justify-center gap-2 text-amber-500 mb-4 rounded-md"
      role="alert"
    >
      <Lock size={16} />
      <span className="font-medium">
        This note is being edited by <strong>{userId}</strong>. Locked for{" "}
        {timeLeft}s.
      </span>
    </div>
  );
}
