import { formatTrend, trendDirection } from "./types";

export function StatCard({
  label,
  value,
  sub,
  trend,
}: {
  label: string;
  value: number | string;
  sub?: string;
  trend?: number | null;
}) {
  const dir = trend !== undefined ? trendDirection(trend ?? null) : null;

  return (
    <div className="stat-card">
      <p className="stat-card-label">{label}</p>
      <p className="stat-card-value">{value}</p>
      {sub && <p className="stat-card-detail">{sub}</p>}
      {dir && trend !== undefined && trend !== null && (
        <p className={`stat-card-trend stat-card-trend--${dir}`}>
          {dir === "up" ? "\u2191" : "\u2193"} {formatTrend(trend)}
        </p>
      )}
    </div>
  );
}
