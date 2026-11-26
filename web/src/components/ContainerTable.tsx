import type { ContainerInfo, Telemetry } from "../api/types.ts";
import { formatBytes, formatPercent, t } from "../i18n/messages.ts";

export interface ContainerTableProps {
  containers: readonly ContainerInfo[];
  telemetry: Telemetry | null;
}

export function ContainerTable({ containers, telemetry }: ContainerTableProps) {
  if (containers.length === 0) {
    return <p className="drawer-empty">{t("drawerNoContainers")}</p>;
  }

  return (
    <div className="table-scroll">
      <table className="container-table">
        <thead>
          <tr>
            <th scope="col">{t("tableName")}</th>
            <th scope="col">{t("tableComposeService")}</th>
            <th scope="col">{t("tableImage")}</th>
            <th scope="col">{t("tableStatus")}</th>
            <th scope="col">{t("tableHealth")}</th>
            <th scope="col" className="num-col">{t("tableCpu")}</th>
            <th scope="col" className="num-col">{t("tableMemory")}</th>
          </tr>
        </thead>
        <tbody>
          {containers.map((ctr) => {
            const stat = telemetry?.containers[ctr.name];
            return (
              <tr key={ctr.id}>
                <td className="mono-value">{ctr.name}</td>
                <td className="mono-value">{ctr.composeService}</td>
                <td className="mono-value image-cell" title={ctr.image}>
                  {ctr.image}
                </td>
                <td>
                  <span className="status-inline">
                    <span>{ctr.status}</span>
                    {ctr.completed && (
                      <span className="oneshot-badge">{t("badgeOneShotCompleted")}</span>
                    )}
                  </span>
                </td>
                <td>{ctr.health}</td>
                <td className="mono-value num-col">
                  {stat ? formatPercent(stat.cpuPct) : "—"}
                </td>
                <td className="mono-value num-col">
                  {stat ? formatBytes(stat.memUsedBytes) : "—"}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
