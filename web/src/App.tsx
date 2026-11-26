import { useCallback, useEffect, useState } from "react";

type HealthState =
  | { kind: "loading" }
  | { kind: "ready"; status: string }
  | { kind: "error"; detail: string };

const messages = {
  en: {
    appTitle: "Docker Shared Services",
    appSubtitle: "Local control plane for shared containers and project stacks.",
    healthLabel: "Backend health",
    endpointLabel: "Endpoint",
    statusChecking: "Checking",
    statusOk: "Connected",
    statusError: "Unavailable",
    retryAction: "Check again",
    errorFallback: "Could not reach the local server.",
  },
} as const;

type Locale = keyof typeof messages;
type MessageKey = keyof (typeof messages)[Locale];

function t(key: MessageKey, locale: Locale = "en"): string {
  return messages[locale][key];
}

export function App() {
  const [health, setHealth] = useState<HealthState>({ kind: "loading" });

  const checkHealth = useCallback(async (signal?: AbortSignal) => {
    setHealth({ kind: "loading" });
    try {
      const response = await fetch("/api/health", { signal });
      if (!response.ok) {
        setHealth({ kind: "error", detail: `HTTP ${response.status}` });
        return;
      }
      const data = (await response.json()) as { status?: string };
      setHealth({ kind: "ready", status: data.status ?? "ok" });
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        return;
      }
      setHealth({ kind: "error", detail: t("errorFallback") });
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void checkHealth(controller.signal);
    return () => controller.abort();
  }, [checkHealth]);

  const statusText =
    health.kind === "loading"
      ? t("statusChecking")
      : health.kind === "ready"
        ? `${t("statusOk")} (${health.status})`
        : `${t("statusError")}: ${health.detail}`;

  return (
    <main className="app-shell">
      <header className="app-header">
        <h1 className="app-title">{t("appTitle")}</h1>
        <p className="app-subtitle">{t("appSubtitle")}</p>
      </header>

      <section className="health-panel" aria-live="polite">
        <div className="health-row">
          <span className="health-label">{t("healthLabel")}</span>
          <span
            className={
              health.kind === "ready" ? "health-badge health-badge--ok" : "health-badge"
            }
          >
            {statusText}
          </span>
        </div>

        <div className="health-row">
          <span className="health-label">{t("endpointLabel")}</span>
          <code className="mono-value">/api/health</code>
        </div>

        <div className="health-actions">
          <button
            type="button"
            className="btn btn--primary"
            onClick={() => void checkHealth()}
          >
            {t("retryAction")}
          </button>
        </div>
      </section>
    </main>
  );
}
