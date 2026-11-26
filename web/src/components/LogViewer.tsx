import { useEffect, useMemo, useRef, useState } from "react";
import type { ServiceDTO } from "../api/types.ts";
import { useLogStream } from "../hooks/useLogStream.ts";
import { t } from "../i18n/messages.ts";

export interface LogViewerProps {
  service: ServiceDTO;
}

export function LogViewer({ service }: LogViewerProps) {
  const [selectedContainer, setSelectedContainer] = useState("");
  const [filterText, setFilterText] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const outputRef = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    setSelectedContainer("");
    setFilterText("");
  }, [service.name]);

  const {
    lines,
    paused,
    bufferedCount,
    error,
    connectionStatus,
    togglePause,
    clear,
  } = useLogStream({
    serviceName: service.name,
    container: selectedContainer,
    tail: 200,
    enabled: true,
  });

  const filteredLines = useMemo(() => {
    const needle = filterText.trim().toLowerCase();
    if (!needle) {
      return lines;
    }
    return lines.filter((line) => line.toLowerCase().includes(needle));
  }, [lines, filterText]);

  useEffect(() => {
    if (!autoScroll || paused || !outputRef.current) {
      return;
    }
    outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [filteredLines, autoScroll, paused]);

  const handleCopy = async () => {
    if (
      filteredLines.length === 0 ||
      typeof navigator === "undefined" ||
      !navigator.clipboard
    ) {
      return;
    }
    try {
      await navigator.clipboard.writeText(filteredLines.join("\n"));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Ignore clipboard write errors in restricted contexts.
    }
  };

  const handleDownload = () => {
    if (filteredLines.length === 0) {
      return;
    }
    const blob = new Blob([`${filteredLines.join("\n")}\n`], {
      type: "text/plain;charset=utf-8",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${service.name}.log`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const pauseLabel = paused
    ? bufferedCount > 0
      ? t("logsResumeBuffered", { count: bufferedCount })
      : t("logsResumeAction")
    : t("logsPauseAction");

  return (
    <div className="log-viewer">
      <div className="log-toolbar">
        <div className="log-toolbar-row">
          <label className="log-field">
            <span className="sr-only">{t("logsContainerLabel")}</span>
            <select
              className="log-select"
              value={selectedContainer}
              onChange={(e) => setSelectedContainer(e.target.value)}
              aria-label={t("logsContainerLabel")}
            >
              <option value="">{t("logsAllContainers")}</option>
              {service.containers.map((ctr) => (
                <option key={ctr.id || ctr.name} value={ctr.name}>
                  {ctr.name}
                </option>
              ))}
            </select>
          </label>

          <input
            type="search"
            className="log-filter-input"
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            placeholder={t("logsFilterPlaceholder")}
            aria-label={t("logsFilterPlaceholder")}
          />

          <span className={`conn-pill conn-pill--${connectionStatus}`}>
            <span className="conn-dot" aria-hidden="true" />
            <span>
              {connectionStatus === "connected"
                ? t("connectionConnected")
                : connectionStatus === "connecting"
                  ? t("connectionConnecting")
                  : t("connectionDisconnected")}
            </span>
          </span>
        </div>

        <div className="log-toolbar-row">
          <button
            type="button"
            className={paused ? "btn btn--primary btn--sm" : "btn btn--secondary btn--sm"}
            onClick={togglePause}
          >
            {pauseLabel}
          </button>

          <label className="ops-autoscroll-toggle">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
            />
            <span>{t("logsAutoScrollLabel")}</span>
          </label>

          <button
            type="button"
            className="btn btn--ghost btn--sm"
            disabled={filteredLines.length === 0}
            onClick={() => void handleCopy()}
          >
            {copied ? t("logsCopiedAction") : t("logsCopyAction")}
          </button>

          <button
            type="button"
            className="btn btn--ghost btn--sm"
            disabled={filteredLines.length === 0}
            onClick={handleDownload}
          >
            {t("logsDownloadAction")}
          </button>

          <button
            type="button"
            className="btn btn--ghost btn--sm"
            disabled={lines.length === 0}
            onClick={clear}
          >
            {t("logsClearAction")}
          </button>
        </div>
      </div>

      {error && (
        <div className="log-error" role="alert">
          {error}
        </div>
      )}

      {lines.length === 0 ? (
        <p className="log-empty">{t("logsEmpty")}</p>
      ) : filteredLines.length === 0 ? (
        <p className="log-empty">{t("logsNoMatches")}</p>
      ) : (
        <pre ref={outputRef} className="log-output mono-value">
          {filteredLines.join("\n")}
        </pre>
      )}
    </div>
  );
}
