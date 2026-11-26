import { useId, useState } from "react";
import { formatPercent, t } from "../i18n/messages.ts";

export interface SparklineProps {
  values: readonly number[];
  maxValue?: number;
  width?: number;
  height?: number;
}

export function Sparkline({
  values,
  maxValue = 100,
  width = 160,
  height = 36,
}: SparklineProps) {
  const gradientId = useId();
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);

  const padX = 3;
  const padY = 4;
  const innerW = width - padX * 2;
  const innerH = height - padY * 2;

  const peak = Math.max(maxValue, ...values, 1);

  const points = values.map((val, idx) => {
    const x =
      values.length <= 1
        ? padX + innerW / 2
        : padX + (idx / (values.length - 1)) * innerW;
    const clamped = Math.max(0, Math.min(val, peak));
    const y = padY + innerH - (clamped / peak) * innerH;
    return { x, y, val };
  });

  const linePath =
    points.length > 1
      ? points.map((p, idx) => `${idx === 0 ? "M" : "L"} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(" ")
      : "";

  const areaPath =
    points.length > 1
      ? `${linePath} L ${points[points.length - 1].x.toFixed(1)} ${(padY + innerH).toFixed(1)} L ${points[0].x.toFixed(1)} ${(padY + innerH).toFixed(1)} Z`
      : "";

  const activePoint = hoveredIndex !== null ? points[hoveredIndex] : null;
  const bandWidth = points.length > 0 ? innerW / points.length : innerW;

  return (
    <div className="sparkline-wrap">
      <svg
        className="sparkline-svg"
        viewBox={`0 0 ${width} ${height}`}
        width={width}
        height={height}
        role="img"
        aria-label={t("cpuHistoryLabel")}
        onMouseLeave={() => setHoveredIndex(null)}
      >
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-accent)" stopOpacity="0.22" />
            <stop offset="100%" stopColor="var(--color-accent)" stopOpacity="0.02" />
          </linearGradient>
        </defs>

        <line
          x1={padX}
          y1={padY + innerH}
          x2={width - padX}
          y2={padY + innerH}
          stroke="var(--color-rule)"
          strokeWidth="1"
        />

        {areaPath && <path d={areaPath} fill={`url(#${gradientId})`} />}

        {linePath && (
          <path
            d={linePath}
            fill="none"
            stroke="var(--color-accent)"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        )}

        {points.length === 1 && (
          <circle
            cx={points[0].x}
            cy={points[0].y}
            r="3"
            fill="var(--color-accent)"
          />
        )}

        {activePoint && (
          <>
            <line
              x1={activePoint.x}
              y1={padY}
              x2={activePoint.x}
              y2={padY + innerH}
              stroke="var(--color-muted)"
              strokeWidth="1"
              strokeDasharray="2 2"
            />
            <circle
              cx={activePoint.x}
              cy={activePoint.y}
              r="3.5"
              fill="var(--light)"
              stroke="var(--color-accent)"
              strokeWidth="2"
            />
          </>
        )}

        {points.map((p, idx) => (
          <rect
            key={idx}
            x={Math.max(0, p.x - bandWidth / 2)}
            y={0}
            width={Math.max(bandWidth, 10)}
            height={height}
            fill="transparent"
            onMouseEnter={() => setHoveredIndex(idx)}
          />
        ))}
      </svg>

      {activePoint !== null && hoveredIndex !== null && (
        <div className="sparkline-tooltip mono-value" role="status">
          {t("cpuSampleTooltip", {
            index: hoveredIndex + 1,
            value: formatPercent(activePoint.val),
          })}
        </div>
      )}
    </div>
  );
}
