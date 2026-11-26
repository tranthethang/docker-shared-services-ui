import type { Telemetry } from "../api/types.ts";
import { formatBytes, formatPercent, t } from "../i18n/messages.ts";
import { Sparkline } from "./Sparkline.tsx";

export interface TelemetryBarProps {
  telemetry: Telemetry | null;
  cpuHistory: readonly number[];
  loading: boolean;
}

export function TelemetryBar({ telemetry, cpuHistory, loading }: TelemetryBarProps) {
  if (!telemetry) {
    return (
      <section className="telemetry-bar telemetry-bar--empty" aria-label={t("telemetryTitle")}>
        <p className="telemetry-placeholder">
          {loading ? t("telemetryLoading") : t("telemetryUnavailable")}
        </p>
      </section>
    );
  }

  const { capacity, diskUsage, totalCpuPct, totalMemBytes } = telemetry;
  const memRatioPct =
    capacity.memTotalBytes > 0
      ? Math.min(100, (totalMemBytes / capacity.memTotalBytes) * 100)
      : 0;

  return (
    <section className="telemetry-bar" aria-label={t("telemetryTitle")}>
      <div className="telemetry-tile">
        <div className="telemetry-tile-head">
          <span className="telemetry-label">{t("cpuLabel")}</span>
          <span className="telemetry-sub mono-value">
            {t("cpuCoresSuffix", { ncpu: capacity.ncpu })}
          </span>
        </div>
        <div className="telemetry-cpu-row">
          <span className="telemetry-value mono-value">{formatPercent(totalCpuPct)}</span>
          <Sparkline values={cpuHistory} />
        </div>
      </div>

      <div className="telemetry-tile">
        <div className="telemetry-tile-head">
          <span className="telemetry-label">{t("memoryLabel")}</span>
          <span className="telemetry-sub mono-value">{formatPercent(memRatioPct)}</span>
        </div>
        <div className="telemetry-value mono-value">
          {t("memoryOfLimit", {
            used: formatBytes(totalMemBytes),
            total: formatBytes(capacity.memTotalBytes),
          })}
        </div>
        <div className="meter-track" aria-hidden="true">
          <div
            className="meter-fill"
            style={{ transform: `scaleX(${(memRatioPct / 100).toFixed(3)})` }}
          />
        </div>
      </div>

      <div className="telemetry-tile telemetry-tile--disk">
        <div className="telemetry-tile-head">
          <span className="telemetry-label">{t("diskLabel")}</span>
          <span className="telemetry-sub mono-value">
            {t("diskTotal", { total: formatBytes(diskUsage.totalBytes) })}
          </span>
        </div>
        <dl className="disk-grid">
          <div className="disk-item">
            <dt>{t("diskImages")}</dt>
            <dd className="mono-value">{formatBytes(diskUsage.imagesBytes)}</dd>
          </div>
          <div className="disk-item">
            <dt>{t("diskContainers")}</dt>
            <dd className="mono-value">{formatBytes(diskUsage.containersBytes)}</dd>
          </div>
          <div className="disk-item">
            <dt>{t("diskVolumes")}</dt>
            <dd className="mono-value">{formatBytes(diskUsage.volumesBytes)}</dd>
          </div>
          <div className="disk-item">
            <dt>{t("diskReclaimable")}</dt>
            <dd className="mono-value">{formatBytes(diskUsage.reclaimableBytes)}</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}
