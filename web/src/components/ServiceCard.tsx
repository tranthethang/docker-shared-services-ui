import type { ServiceAction, ServiceDTO, Telemetry } from "../api/types.ts";
import { formatBytes, formatPercent, t, type MessageKey } from "../i18n/messages.ts";
import { ServiceActions } from "./ServiceActions.tsx";

export interface ServiceCardProps {
  service: ServiceDTO;
  telemetry: Telemetry | null;
  selected: boolean;
  onSelect: (name: string) => void;
  checked?: boolean;
  onToggleChecked?: (name: string) => void;
  opsBusy?: boolean;
  onAction?: (serviceName: string, action: ServiceAction) => void;
}

const STATE_LABEL_KEYS: Record<string, MessageKey> = {
  running: "stateRunning",
  partial: "statePartial",
  stopped: "stateStopped",
  absent: "stateAbsent",
};

const HEALTH_LABEL_KEYS: Record<string, MessageKey> = {
  healthy: "healthHealthy",
  starting: "healthStarting",
  unhealthy: "healthUnhealthy",
  none: "healthNone",
  "-": "healthDash",
};

function getServiceResources(
  service: ServiceDTO,
  telemetry: Telemetry | null,
): { cpuPct: number; memBytes: number; hasStats: boolean } {
  if (!telemetry) {
    return { cpuPct: 0, memBytes: 0, hasStats: false };
  }
  let cpuPct = 0;
  let memBytes = 0;
  let hasStats = false;
  for (const ctr of service.containers) {
    const stat = telemetry.containers[ctr.name];
    if (stat) {
      cpuPct += stat.cpuPct;
      memBytes += stat.memUsedBytes;
      hasStats = true;
    }
  }
  return { cpuPct, memBytes, hasStats };
}

export function ServiceCard({
  service,
  telemetry,
  selected,
  onSelect,
  checked = false,
  onToggleChecked,
  opsBusy = false,
  onAction,
}: ServiceCardProps) {
  const stateKey = STATE_LABEL_KEYS[service.state] ?? "stateAbsent";
  const healthKey = HEALTH_LABEL_KEYS[service.health] ?? "healthDash";
  const { cpuPct, memBytes, hasStats } = getServiceResources(service, telemetry);
  const showHealth = service.health !== "-" && service.health !== "none";
  const lockReason = service.lockedReason || t("serviceLockedFallback");
  const isActive = service.state === "running" || service.state === "partial";

  const rowClasses = [
    "service-row",
    isActive ? "service-row--active" : "service-row--inactive",
    service.state === "partial" ? "service-row--partial" : "",
    selected ? "service-row--selected" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <article className={rowClasses} role="listitem" data-state={service.state}>
      <div className="service-row-main">
        {onToggleChecked && (
          <input
            type="checkbox"
            className="service-batch-checkbox"
            checked={checked}
            disabled={opsBusy}
            aria-label={t("selectServiceLabel", { name: service.name })}
            onChange={() => onToggleChecked(service.name)}
          />
        )}

        <span
          className={`service-state-dot service-state-dot--${service.state}`}
          aria-hidden="true"
        />

        <div className="service-row-identity">
          <div className="service-row-title-line">
            <button
              type="button"
              className="service-row-trigger"
              onClick={() => onSelect(service.name)}
              aria-label={t("inspectService", { name: service.name })}
            >
              <span className="service-row-name">{service.name}</span>
            </button>

            <div className="service-badges">
              {service.locked && (
                <span className="locked-pill" title={lockReason}>
                  {t("serviceLockedBadge")}
                </span>
              )}
              <span className={`status-pill status-pill--${service.state}`}>
                {t(stateKey)}
              </span>
              {showHealth && (
                <span className={`health-pill health-pill--${service.health}`}>
                  {t(healthKey)}
                </span>
              )}
            </div>
          </div>

          <p className="service-row-desc">{service.description}</p>
        </div>
      </div>

      <div className="service-row-aside">
        <div className="service-metrics mono-value">
          <span>
            {t("containersCount", {
              running: service.runningCount,
              total: service.containerCount,
            })}
          </span>
          {hasStats && (
            <>
              <span>{formatPercent(cpuPct)}</span>
              <span>{formatBytes(memBytes)}</span>
            </>
          )}
        </div>

        <div className="service-row-controls">
          {service.webUi.length > 0 && (
            <div className="service-links">
              {service.webUi.map((url) => (
                <a
                  key={url}
                  href={url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="service-webui-link"
                >
                  {t("openWebUi")}
                </a>
              ))}
            </div>
          )}

          {onAction && (
            <ServiceActions
              service={service}
              opsBusy={opsBusy}
              onAction={onAction}
              compact
            />
          )}
        </div>
      </div>
    </article>
  );
}
