import type { InfoResponse } from "../api/types.ts";
import type { ConnectionStatus } from "../hooks/useDashboardData.ts";
import type { ThemePreference } from "../hooks/useTheme.ts";
import { t } from "../i18n/messages.ts";

export interface HeaderProps {
  info: InfoResponse | null;
  runningServices: number;
  totalServices: number;
  connectionStatus: ConnectionStatus;
  refreshing: boolean;
  theme: ThemePreference;
  onCycleTheme: () => void;
  onOpenShortcuts: () => void;
  onRefresh: () => void;
}

export function Header({
  info,
  runningServices,
  totalServices,
  connectionStatus,
  refreshing,
  theme,
  onCycleTheme,
  onOpenShortcuts,
  onRefresh,
}: HeaderProps) {
  const themeModeLabel =
    theme === "dark"
      ? t("themeDark")
      : theme === "light"
        ? t("themeLight")
        : t("themeSystem");
  const connectionLabel =
    connectionStatus === "connected"
      ? t("connectionConnected")
      : connectionStatus === "connecting"
        ? t("connectionConnecting")
        : t("connectionDisconnected");

  return (
    <header className="dash-header">
      <div className="dash-header-main">
        <div className="dash-title-group">
          <h1 className="dash-title">{t("appTitle")}</h1>
          <div className="dash-meta-row">
            {info && (
              <>
                <span className="dash-meta-item">
                  <span className="dash-meta-label">{t("stackLabel")}</span>
                  <span className="mono-value">{info.stackName}</span>
                </span>
                <span className="dash-meta-item">
                  <span className="dash-meta-label">{t("domainLabel")}</span>
                  <span className="mono-value">{info.domain}</span>
                </span>
              </>
            )}
            <span className="dash-meta-item">
              <span className="mono-value">
                {t("servicesRunningSummary", {
                  running: runningServices,
                  total: totalServices,
                })}
              </span>
            </span>
          </div>
        </div>

        <div className="dash-header-actions">
          <span
            className={`conn-pill conn-pill--${connectionStatus}`}
            aria-live="polite"
          >
            <span className="conn-dot" aria-hidden="true" />
            <span>{connectionLabel}</span>
          </span>

          <button
            type="button"
            className="btn btn--ghost btn--sm"
            onClick={onCycleTheme}
            title={t("themeToggleLabel", { mode: themeModeLabel })}
          >
            {t("themeToggleLabel", { mode: themeModeLabel })}
          </button>

          <button
            type="button"
            className="btn btn--ghost btn--sm"
            onClick={onOpenShortcuts}
            aria-label={t("shortcutsOpenLabel")}
            title={t("shortcutsOpenLabel")}
          >
            ?
          </button>

          <button
            type="button"
            className="btn btn--secondary"
            onClick={onRefresh}
            disabled={refreshing}
          >
            {refreshing ? t("refreshingAction") : t("refreshAction")}
          </button>
        </div>
      </div>
    </header>
  );
}
