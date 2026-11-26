import { useState } from "react";
import type { EnvStatusResponse } from "../api/admin.ts";
import { postEnvMerge, postSetup } from "../api/admin.ts";
import type { OpsJob } from "../api/types.ts";
import { t } from "../i18n/messages.ts";

interface EnvSetupViewProps {
  status: EnvStatusResponse | null;
  loading: boolean;
  error: string | null;
  opsBusy: boolean;
  onRefresh: () => void;
  onJobStarted: (job: OpsJob) => void;
  onError: (message: string) => void;
}

export function EnvSetupView({
  status,
  loading,
  error,
  opsBusy,
  onRefresh,
  onJobStarted,
  onError,
}: EnvSetupViewProps) {
  const [password, setPassword] = useState("");
  const [force, setForce] = useState(false);
  const [merging, setMerging] = useState(false);
  const [mergeMessage, setMergeMessage] = useState<string | null>(null);

  const handleRunSetup = async () => {
    setMergeMessage(null);
    try {
      const job = await postSetup(password, force);
      onJobStarted(job);
      setPassword("");
    } catch (err: unknown) {
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleMergeMissing = async () => {
    setMerging(true);
    setMergeMessage(null);
    try {
      const res = await postEnvMerge();
      setMergeMessage(t("envMergeSuccess", { count: res.added }));
      onRefresh();
    } catch (err: unknown) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setMerging(false);
    }
  };

  if (loading && !status) {
    return (
      <section className="empty-panel" aria-live="polite">
        <p className="empty-panel-text">{t("envLoading")}</p>
      </section>
    );
  }

  if (error && !status) {
    return (
      <section className="error-banner" role="alert">
        <p className="error-banner-detail">{error}</p>
      </section>
    );
  }

  const missingFiles = status?.files.filter((f) => f.missing) ?? [];
  const problems = status?.problems ?? [];
  const networks = status?.networks ?? [];

  return (
    <div className="admin-view">
      <section className="admin-panel">
        <div className="admin-panel-head">
          <div>
            <h2 className="admin-panel-title">{t("envSetupTitle")}</h2>
            <p className="admin-panel-desc">{t("envSetupSubtitle")}</p>
          </div>
          <div className="admin-actions">
            <button
              type="button"
              className="btn btn--secondary"
              disabled={opsBusy || merging}
              onClick={() => void handleMergeMissing()}
            >
              {t("envMergeMissingAction")}
            </button>
            <button
              type="button"
              className="btn btn--primary"
              disabled={opsBusy || merging}
              onClick={() => void handleRunSetup()}
            >
              {t("envRunSetupAction")}
            </button>
          </div>
        </div>

        <div className="filter-bar">
          <label className="filter-field filter-field--search">
            <span className="filter-label">{t("envOptionalPasswordLabel")}</span>
            <input
              type="password"
              className="filter-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t("envOptionalPasswordPlaceholder")}
              autoComplete="new-password"
            />
          </label>
          <label className="admin-check-item">
            <input
              type="checkbox"
              checked={force}
              onChange={(e) => setForce(e.target.checked)}
            />
            <span>{t("envForcePasswordLabel")}</span>
          </label>
        </div>

        {mergeMessage && (
          <p className="admin-panel-desc" role="status">
            {mergeMessage}
          </p>
        )}
      </section>

      <div className="admin-grid">
        <section className="admin-panel">
          <h3 className="admin-panel-title">{t("envNetworksHeading")}</h3>
          <ul className="admin-list">
            {networks.map((net) => (
              <li key={net.name} className="admin-list-item">
                <div>
                  <div className="mono-value">{net.name}</div>
                  <div className="telemetry-sub mono-value">{net.subnet}</div>
                </div>
                <span
                  className={
                    net.exists
                      ? "status-pill status-pill--running"
                      : "status-pill status-pill--partial"
                  }
                >
                  {net.exists ? t("envNetworkExists") : t("envNetworkMissing")}
                </span>
              </li>
            ))}
          </ul>
        </section>

        <section className="admin-panel">
          <div className="admin-panel-head">
            <h3 className="admin-panel-title">{t("envFilesHeading")}</h3>
            <span className="status-pill">
              {missingFiles.length === 0
                ? t("envFilesAllPresent", { count: status?.files.length ?? 0 })
                : t("envFilesMissingCount", { count: missingFiles.length })}
            </span>
          </div>
          <ul className="admin-list">
            {missingFiles.map((file) => (
              <li key={file.service} className="admin-list-item">
                <span className="mono-value">
                  {file.service === "." ? ".env" : `${file.service}/.env`}
                </span>
                <span className="status-pill status-pill--partial">
                  {t("envFileStatusMissing")}
                </span>
              </li>
            ))}
            {missingFiles.length === 0 && (
              <li className="admin-list-item">
                <span>{t("envFilesAllPresent", { count: status?.files.length ?? 0 })}</span>
                <span className="status-pill status-pill--running">
                  {t("envFileStatusPresent")}
                </span>
              </li>
            )}
          </ul>
        </section>
      </div>

      <section className="admin-panel">
        <h3 className="admin-panel-title">{t("envProblemsHeading")}</h3>
        {problems.length === 0 ? (
          <p className="admin-panel-desc">{t("envNoProblems")}</p>
        ) : (
          <ul className="admin-list">
            {problems.map((prob, idx) => (
              <li key={`${prob.service}-${prob.code}-${idx}`} className="admin-list-item">
                <div>
                  <div className="mono-value">
                    {prob.service}
                    {prob.key ? ` · ${prob.key}` : ""}
                  </div>
                  <div className="admin-panel-desc">{prob.message}</div>
                  {prob.remediation && (
                    <div className="telemetry-sub">{prob.remediation}</div>
                  )}
                </div>
                <span className="status-pill status-pill--partial">{prob.code}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
