import { useState } from "react";
import type { ServiceDTO, Telemetry } from "../api/types.ts";
import { t } from "../i18n/messages.ts";
import { ServiceCard } from "./ServiceCard.tsx";

export interface ServiceGroupProps {
  category: string;
  services: readonly ServiceDTO[];
  runningCount: number;
  totalCount: number;
  telemetry: Telemetry | null;
  selectedServiceName: string | null;
  onSelectService: (name: string) => void;
}

export function ServiceGroup({
  category,
  services,
  runningCount,
  totalCount,
  telemetry,
  selectedServiceName,
  onSelectService,
}: ServiceGroupProps) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <section className="service-group" aria-label={category}>
      <button
        type="button"
        className="service-group-header"
        aria-expanded={!collapsed}
        aria-label={
          collapsed
            ? t("expandGroup", { category })
            : t("collapseGroup", { category })
        }
        onClick={() => setCollapsed((prev) => !prev)}
      >
        <span className="service-group-title-wrap">
          <span
            className={collapsed ? "group-chevron group-chevron--collapsed" : "group-chevron"}
            aria-hidden="true"
          >
            ▾
          </span>
          <h2 className="service-group-title">{category}</h2>
        </span>

        <span className="service-group-count mono-value">
          {t("groupRunningCount", { running: runningCount, total: totalCount })}
        </span>
      </button>

      {!collapsed && (
        <div className="service-grid">
          {services.map((svc) => (
            <ServiceCard
              key={svc.name}
              service={svc}
              telemetry={telemetry}
              selected={selectedServiceName === svc.name}
              onSelect={onSelectService}
            />
          ))}
        </div>
      )}
    </section>
  );
}
