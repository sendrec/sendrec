import { formatChartDate } from "../../utils/format";
import type { DailyViews } from "./types";

export function CssBarChart({ daily }: { daily: DailyViews[] }) {
  if (daily.length === 0) return null;

  const maxViews = Math.max(...daily.map((d) => d.views), 1);
  const yLabels = [maxViews, Math.round(maxViews * 0.75), Math.round(maxViews * 0.5), Math.round(maxViews * 0.25), 0];

  const showEveryNth = daily.length > 14 ? Math.ceil(daily.length / 10) : 1;

  return (
    <div className="analytics-chart">
      <div className="analytics-chart-yaxis">
        {yLabels.map((label, i) => (
          <span key={i}>{label}</span>
        ))}
      </div>
      <div className="analytics-chart-area">
        <div className="analytics-chart-grid">
          {yLabels.map((_, i) => (
            <div key={i} className="analytics-chart-grid-line" />
          ))}
        </div>
        <div className="analytics-chart-bars">
          {daily.map((d, i) => {
            const heightPct =
              maxViews > 0 ? (d.views / maxViews) * 100 : 0;
            return (
              <div key={i} className="analytics-chart-bar-wrapper">
                <div className="analytics-chart-tooltip">
                  {formatChartDate(d.date)}: {d.views} views ({d.uniqueViews}{" "}
                  unique)
                </div>
                <div
                  className="analytics-chart-bar"
                  style={{ height: `${heightPct}%` }}
                />
              </div>
            );
          })}
        </div>
        <div className="analytics-chart-xaxis">
          {daily.map((d, i) => (
            <span key={i}>
              {i % showEveryNth === 0 ? formatChartDate(d.date) : ""}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
