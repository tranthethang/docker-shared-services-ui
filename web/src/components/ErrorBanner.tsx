import type { DashboardError } from "../hooks/useDashboardData.ts";
import { t } from "../i18n/messages.ts";

export interface ErrorBannerProps {
  error: DashboardError;
  onRetry: () => void;
  retrying: boolean;
}

export function ErrorBanner({ error, onRetry, retrying }: ErrorBannerProps) {
  const detail =
    error.code === "docker_unavailable"
      ? t("errorDockerUnavailable")
      : error.message;

  return (
    <section className="error-banner" role="alert" aria-live="assertive">
      <div className="error-banner-copy">
        <h2 className="error-banner-title">{t("errorBannerTitle")}</h2>
        <p className="error-banner-detail">{detail}</p>
      </div>
      <button
        type="button"
        className="btn btn--primary"
        onClick={onRetry}
        disabled={retrying}
      >
        {retrying ? t("refreshingAction") : t("errorRetryAction")}
      </button>
    </section>
  );
}
