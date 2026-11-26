import { useEffect } from "react";
import type { ServiceDTO, Telemetry } from "../api/types.ts";
import { t, type MessageKey } from "../i18n/messages.ts";
import { ContainerTable } from "./ContainerTable.tsx";

export interface ServiceDrawerProps {
  service: ServiceDTO | null;
  telemetry: Telemetry | null;
  onClose: () => void;
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

export function ServiceDrawer({ service, telemetry, onClose }: ServiceDrawerProps) {
  useEffect(() => {
    if (!service) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [service, onClose]);

  if (!service) {
    return null;
  }

  const stateKey = STATE_LABEL_KEYS[service.state] ?? "stateAbsent";
  const healthKey = HEALTH_LABEL_KEYS[service.health] ?? "healthDash";

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <aside
        className="service-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="drawer-service-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="drawer-header">
          <div>
            <h2 id="drawer-service-title" className="drawer-title">
              {service.name}
            </h2>
            <p className="drawer-desc">{service.description}</p>
          </div>
          <button type="button" className="btn btn--secondary" onClick={onClose}>
            {t("drawerClose")}
          </button>
        </div>

        <dl className="drawer-meta">
          <div className="drawer-meta-item">
            <dt>{t("drawerCategoryLabel")}</dt>
            <dd>{service.category}</dd>
          </div>
          <div className="drawer-meta-item">
            <dt>{t("drawerStateLabel")}</dt>
            <dd>
              <span className={`status-pill status-pill--${service.state}`}>
                {t(stateKey)}
              </span>
            </dd>
          </div>
          <div className="drawer-meta-item">
            <dt>{t("drawerHealthLabel")}</dt>
            <dd>
              <span className={`health-pill health-pill--${service.health}`}>
                {t(healthKey)}
              </span>
            </dd>
          </div>
        </dl>

        {service.webUi.length > 0 && (
          <section className="drawer-section">
            <h3 className="drawer-section-title">{t("drawerWebUiLabel")}</h3>
            <ul className="drawer-link-list">
              {service.webUi.map((url) => (
                <li key={url}>
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="drawer-external-link mono-value"
                  >
                    {url}
                  </a>
                </li>
              ))}
            </ul>
          </section>
        )}

        {service.info.length > 0 && (
          <section className="drawer-section">
            <h3 className="drawer-section-title">{t("drawerInfoLabel")}</h3>
            <ul className="drawer-info-list">
              {service.info.map((note) => (
                <li key={note} className="mono-value">
                  {note}
                </li>
              ))}
            </ul>
          </section>
        )}

        {service.containers.length === 0 && service.expectedContainers.length > 0 && (
          <section className="drawer-section">
            <h3 className="drawer-section-title">{t("drawerExpectedContainersLabel")}</h3>
            <ul className="drawer-info-list">
              {service.expectedContainers.map((name) => (
                <li key={name} className="mono-value">
                  {name}
                </li>
              ))}
            </ul>
          </section>
        )}

        <section className="drawer-section">
          <h3 className="drawer-section-title">{t("drawerContainersLabel")}</h3>
          <ContainerTable containers={service.containers} telemetry={telemetry} />
        </section>
      </aside>
    </div>
  );
}
