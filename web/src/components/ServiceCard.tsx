import type { ServiceDTO, Telemetry } from "../api/types.ts";
import { formatBytes, formatPercent, t, type MessageKey } from "../i18n/messages.ts";

export interface ServiceCardProps {
  service: ServiceDTO;
  telemetry: Telemetry | null;
  selected: boolean;
  onSelect: (name: string) => void;
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

export function ServiceCard({ service, telemetry, selected, onSelect }: ServiceCardProps) {
  const stateKey = STATE_LABEL_KEYS[service.state] ?? "stateAbsent";
  const healthKey = HEALTH_LABEL_KEYS[service.health] ?? "healthDash";
  const { cpuPct, memBytes, hasStats } = getServiceResources(service, telemetry);
  const showHealth = service.health !== "-" && service.health !== "none";

  return (
    <article
      className={selected ? "service-card service-card--selected" : "service-card"}
    >
      <div className="service-card-top">
        <button
          type="button"
          className="service-card-trigger"
          onClick={() => onSelect(service.name)}
          aria-label={t("inspectService", { name: service.name })}
        >
          <span className="service-card-name">{service.name}</span>
        </button>

        <div className="service-badges">
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

      <p className="service-card-desc">{service.description}</p>

      <div className="service-card-foot">
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
      </div>
    </article>
  );
}
