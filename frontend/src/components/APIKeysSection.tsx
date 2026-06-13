import { useState, useEffect, useCallback } from "react";
import { Copy, Check, Loader2, Key } from "lucide-react";
import { useAuth } from "@/context/AuthContext";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  issueAPIKey,
  getAPIKeyStatus,
  revokeAPIKey,
  APIKeyStatus,
} from "@/lib/api";

type ConfirmKind = "regenerate" | "revoke" | null;

export function APIKeysSection() {
  const { user } = useAuth();
  const isDemo = !!user?.id.startsWith("demo-user-");
  const [status, setStatus] = useState<APIKeyStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmKind, setConfirmKind] = useState<ConfirmKind>(null);

  const fetchStatus = useCallback(async () => {
    if (isDemo) return;
    try {
      const s = await getAPIKeyStatus();
      setStatus(s);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Failed to load API key status",
      );
    } finally {
      setLoading(false);
    }
  }, [isDemo]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchStatus();
  }, [fetchStatus]);

  if (isDemo) return null;

  const doIssue = async () => {
    setBusy(true);
    setError(null);
    try {
      const result = await issueAPIKey();
      setNewKey(result.key);
      await fetchStatus();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to issue API key");
    } finally {
      setBusy(false);
    }
  };

  const handleIssue = () => {
    if (status?.has_key) {
      setConfirmKind("regenerate");
    } else {
      doIssue();
    }
  };

  const handleRevoke = () => setConfirmKind("revoke");

  const handleConfirm = async () => {
    setConfirmKind(null);
    if (confirmKind === "regenerate") {
      await doIssue();
    } else if (confirmKind === "revoke") {
      setBusy(true);
      setError(null);
      try {
        await revokeAPIKey();
        await fetchStatus();
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to revoke API key");
      } finally {
        setBusy(false);
      }
    }
  };

  const handleCopy = async () => {
    if (!newKey) return;
    await navigator.clipboard.writeText(newKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const formatDate = (epoch: number) =>
    new Date(epoch * 1000).toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });

  const cardStyle = {
    background: "var(--card)",
    padding: "1.5rem",
    borderRadius: "0.5rem",
    border: "1px solid var(--border)",
    marginBottom: "2rem",
  } as const;

  const headingStyle = {
    fontSize: "1.125rem",
    fontWeight: 600,
    marginBottom: "0.5rem",
  } as const;

  const confirmMessages: Record<
    NonNullable<ConfirmKind>,
    { title: string; message: string; label: string }
  > = {
    regenerate: {
      title: "Regenerate API Key",
      message:
        "This will immediately revoke your existing key. Any clients using the old key will stop working.",
      label: "Regenerate",
    },
    revoke: {
      title: "Revoke API Key",
      message: "Are you sure you want to revoke your API key?",
      label: "Revoke",
    },
  };

  const confirm = confirmKind ? confirmMessages[confirmKind] : null;

  return (
    <div style={cardStyle}>
      <h2 style={headingStyle}>API Keys</h2>
      <p
        style={{
          color: "var(--muted-foreground)",
          fontSize: "0.875rem",
          marginBottom: "1rem",
        }}
      >
        Programmatic access to GophDrive (e.g. gophmem CLI). One key per
        account. The plaintext key is shown once — copy it before closing.
      </p>

      {loading && (
        <Loader2
          className="animate-spin"
          size={16}
          style={{ color: "var(--muted-foreground)" }}
        />
      )}

      {!loading && error && (
        <p style={{ color: "var(--destructive)", fontSize: "0.875rem" }}>
          {error}
        </p>
      )}

      {newKey && (
        <div
          style={{
            background: "var(--muted)",
            border: "1px solid var(--border)",
            borderRadius: "0.375rem",
            padding: "1rem",
            marginBottom: "1rem",
          }}
        >
          <p
            style={{
              color: "var(--destructive)",
              fontSize: "0.8rem",
              fontWeight: 600,
              marginBottom: "0.5rem",
            }}
          >
            Copy your key now — it will not be shown again.
          </p>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              marginBottom: "0.75rem",
            }}
          >
            <code
              style={{
                flex: 1,
                background: "var(--background)",
                padding: "0.375rem 0.5rem",
                borderRadius: "0.25rem",
                fontFamily: "monospace",
                fontSize: "0.8rem",
                overflowX: "auto",
                wordBreak: "break-all",
              }}
            >
              {newKey}
            </code>
            <button
              onClick={handleCopy}
              className="btn"
              title="Copy key"
              style={{
                padding: "0.375rem",
                background: "transparent",
                border: "1px solid var(--border)",
                borderRadius: "0.25rem",
                cursor: "pointer",
                flexShrink: 0,
              }}
            >
              {copied ? (
                <Check size={16} style={{ color: "var(--foreground)" }} />
              ) : (
                <Copy size={16} style={{ color: "var(--foreground)" }} />
              )}
            </button>
          </div>
          <p
            style={{
              fontSize: "0.75rem",
              color: "var(--muted-foreground)",
            }}
          >
            Set in your shell:{" "}
            <code style={{ fontFamily: "monospace" }}>
              export GOPHMEM_API_KEY=&lt;key&gt;
            </code>
          </p>
          <button
            onClick={() => setNewKey(null)}
            className="btn"
            style={{
              marginTop: "0.75rem",
              background: "transparent",
              border: "1px solid var(--border)",
              color: "var(--foreground)",
              fontSize: "0.875rem",
              cursor: "pointer",
            }}
          >
            Done — I have copied my key
          </button>
        </div>
      )}

      {!loading && !newKey && status && (
        <div style={{ marginBottom: "1rem" }}>
          {status.has_key ? (
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.75rem",
                flexWrap: "wrap",
              }}
            >
              <Key size={16} style={{ color: "var(--muted-foreground)" }} />
              <code
                style={{
                  fontFamily: "monospace",
                  fontSize: "0.875rem",
                  color: "var(--foreground)",
                }}
              >
                {status.key_prefix}••••••••
              </code>
              {status.first_issued_at &&
              status.created_at &&
              status.first_issued_at !== status.created_at ? (
                <span
                  style={{
                    fontSize: "0.8rem",
                    color: "var(--muted-foreground)",
                  }}
                >
                  Issued {formatDate(status.first_issued_at)} · Last rotated{" "}
                  {formatDate(status.created_at)}
                </span>
              ) : status.created_at ? (
                <span
                  style={{
                    fontSize: "0.8rem",
                    color: "var(--muted-foreground)",
                  }}
                >
                  Issued {formatDate(status.created_at)}
                </span>
              ) : null}
            </div>
          ) : (
            <p
              style={{
                fontSize: "0.875rem",
                color: "var(--muted-foreground)",
              }}
            >
              No API key issued.
            </p>
          )}
        </div>
      )}

      {!loading && (
        <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
          <button
            onClick={handleIssue}
            disabled={busy}
            className="btn"
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "0.5rem",
              background: "transparent",
              color: "var(--foreground)",
              borderColor: "var(--border)",
              opacity: busy ? 0.6 : 1,
              cursor: busy ? "not-allowed" : "pointer",
            }}
          >
            {busy ? <Loader2 size={14} className="animate-spin" /> : null}
            {status?.has_key ? "Regenerate Key" : "Issue Key"}
          </button>

          {status?.has_key && (
            <button
              onClick={handleRevoke}
              disabled={busy}
              className="btn"
              style={{
                background: "transparent",
                color: "var(--destructive)",
                borderColor: "var(--destructive)",
                opacity: busy ? 0.6 : 1,
                cursor: busy ? "not-allowed" : "pointer",
              }}
            >
              Revoke
            </button>
          )}
        </div>
      )}

      {confirm && (
        <ConfirmDialog
          isOpen
          title={confirm.title}
          message={confirm.message}
          confirmLabel={confirm.label}
          onConfirm={handleConfirm}
          onCancel={() => setConfirmKind(null)}
        />
      )}
    </div>
  );
}
