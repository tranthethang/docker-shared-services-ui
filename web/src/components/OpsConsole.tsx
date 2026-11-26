import { useEffect, useRef, useState } from "react";
import type { OpsJob } from "../api/types.ts";
import { t } from "../i18n/messages.ts";

export interface OpsConsoleProps {
  busy: boolean;
  currentJob: OpsJob | null;
  lastJob: OpsJob | null;
  lines: readonly string[];
  expanded: boolean;
  onToggleExpanded: () => void;
  onClear: () => void;
}

function getJobSummary(job: OpsJob | null): string {
  if (!job) {
    return "";
  }
  if (job.service) {
    return `${job.service}: ${job.action}`;
  }
  return job.action;
}

export function OpsConsole({
  busy,
  currentJob,
  lastJob,
  lines,
  expanded,
  onToggleExpanded,
  onClear,
}: OpsConsoleProps) {
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const bodyRef = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    if (!expanded || !autoScroll || !bodyRef.current) {
      return;
    }
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
  }, [lines, expanded, autoScroll]);

  const activeOrLast = currentJob ?? lastJob;
  const statusKey = busy
    ? "opsStatusRunning"
    : lastJob?.status === "succeeded"
      ? "opsStatusSucceeded"
      : lastJob?.status === "failed"
        ? "opsStatusFailed"
        : "opsStatusIdle";

  const statusTone = busy
    ? "running"
    : lastJob?.status === "succeeded"
      ? "succeeded"
      : lastJob?.status === "failed"
        ? "failed"
        : "idle";

  const handleCopy = async () => {
    if (lines.length === 0 || typeof navigator === "undefined" || !navigator.clipboard) {
      return;
    }
    try {
      await navigator.clipboard.writeText(lines.join("\n"));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Ignore clipboard write errors in restricted contexts.
    }
  };

  return (
    <section
      className={expanded ? "ops-console ops-console--expanded" : "ops-console"}
      aria-label={t("opsConsoleTitle")}
    >
      <div className="ops-console-bar">
        <button
          type="button"
          className="ops-console-toggle"
          aria-expanded={expanded}
          aria-label={expanded ? t("opsCollapseAction") : t("opsExpandAction")}
          onClick={onToggleExpanded}
        >
          <span
            className={expanded ? "group-chevron" : "group-chevron group-chevron--collapsed"}
            aria-hidden="true"
          >
            ▾
          </span>
          <span className="ops-console-title">{t("opsConsoleTitle")}</span>
          <span className={`ops-status-pill ops-status-pill--${statusTone}`}>
            {t(statusKey)}
          </span>
          {activeOrLast && (
            <span className="ops-console-job mono-value">
              {getJobSummary(activeOrLast)}
            </span>
          )}
        </button>

        {expanded && (
          <div className="ops-console-controls">
            <label className="ops-autoscroll-toggle">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
              />
              <span>{t("opsAutoScrollLabel")}</span>
            </label>

            <button
              type="button"
              className="btn btn--ghost btn--sm"
              disabled={lines.length === 0}
              onClick={() => void handleCopy()}
            >
              {copied ? t("opsCopiedAction") : t("opsCopyAction")}
            </button>

            <button
              type="button"
              className="btn btn--ghost btn--sm"
              disabled={lines.length === 0}
              onClick={onClear}
            >
              {t("opsClearAction")}
            </button>
          </div>
        )}
      </div>

      {expanded && (
        <div className="ops-console-body-wrap">
          {lastJob && !busy && lastJob.message && (
            <div className={`ops-console-result ops-console-result--${statusTone}`}>
              {lastJob.message}
            </div>
          )}

          {lines.length === 0 ? (
            <p className="ops-console-empty">{t("opsEmptyOutput")}</p>
          ) : (
            <pre ref={bodyRef} className="ops-console-output mono-value">
              {lines.join("\n")}
            </pre>
          )}
        </div>
      )}
    </section>
  );
}
