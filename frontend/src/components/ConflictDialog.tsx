"use client";

import React from "react";
import { AlertCircle } from "lucide-react";

interface ConflictDialogProps {
  isOpen: boolean;
  onKeepLocal: () => void;
  onKeepRemote: () => void;
}

export function ConflictDialog({
  isOpen,
  onKeepLocal,
  onKeepRemote,
}: ConflictDialogProps) {
  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      style={{
        backgroundColor: "rgba(0,0,0,0.5)",
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 50,
      }}
    >
      <div
        className="glass p-6 rounded-xl max-w-md w-full border border-red-500/30"
        style={{
          padding: "1.5rem",
          borderRadius: "0.75rem",
          width: "100%",
          maxWidth: "28rem",
          border: "1px solid rgba(239, 68, 68, 0.3)",
        }}
      >
        <h2
          className="text-xl font-bold mb-4 flex items-center gap-2"
          style={{
            fontSize: "1.25rem",
            fontWeight: 700,
            marginBottom: "1rem",
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            color: "#f87171",
          }}
        >
          <AlertCircle />
          Conflict Detected
        </h2>
        <p
          className="mb-6 opacity-80"
          style={{ marginBottom: "1.5rem", opacity: 0.8 }}
        >
          The file has been modified on the server since you started editing.
          Which version do you want to keep?
        </p>
        <div
          className="flex gap-4 justify-end"
          style={{ display: "flex", gap: "1rem", justifyContent: "flex-end" }}
        >
          <button
            onClick={onKeepRemote}
            className="btn"
            style={{ background: "#374151", color: "white" }}
          >
            Discard Local Changes
          </button>
          <button
            onClick={onKeepLocal}
            className="btn"
            style={{ background: "#ef4444", color: "white" }}
          >
            Overwrite Remote
          </button>
        </div>
      </div>
    </div>
  );
}
