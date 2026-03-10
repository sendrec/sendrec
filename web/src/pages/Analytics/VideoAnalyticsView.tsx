import { formatChartDate } from "../../utils/format";
import type { AnalyticsData, Range, SortColumn, SortDirection } from "./types";
import { RANGE_SUBTITLES } from "./types";
import { StatCard } from "./StatCard";
import { CssBarChart } from "./CssBarChart";
import { ViewerTable } from "./ViewerTable";

export function VideoAnalyticsView({
  data,
  range,
  sortColumn,
  sortDirection,
  visibleViewerCount,
  onSort,
  onShowMore,
  sortIndicator,
}: {
  data: AnalyticsData;
  range: Range;
  sortColumn: SortColumn;
  sortDirection: SortDirection;
  visibleViewerCount: number;
  onSort: (column: SortColumn) => void;
  onShowMore: () => void;
  sortIndicator: (column: SortColumn) => string;
}) {
  const hasViews = data.summary.totalViews > 0;
  const trends = range !== "all" ? data.trends : null;

  return (
    <>
      <div className="analytics-stats">
        <StatCard
          label="Total Views"
          value={data.summary.totalViews}
          trend={trends?.views ?? null}
        />
        <StatCard
          label="Unique Views"
          value={data.summary.uniqueViews}
          trend={trends?.uniqueViews ?? null}
        />
        <StatCard
          label="Avg / Day"
          value={data.summary.averageDailyViews}
        />
        <StatCard
          label="Peak Day"
          value={data.summary.peakDayViews}
          sub={data.summary.peakDay ? formatChartDate(data.summary.peakDay) : undefined}
        />
        <StatCard
          label="CTA Clicks"
          value={data.summary.totalCtaClicks}
          sub={
            data.summary.totalViews > 0
              ? `${(data.summary.ctaClickRate * 100).toFixed(1)}% click rate`
              : undefined
          }
        />
      </div>

      {!hasViews && (
        <div className="card">
          <div className="empty-state">
            <div className="empty-state-icon">
              <svg
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M18 20V10M12 20V4M6 20v-6" />
              </svg>
            </div>
            <div className="empty-state-title">No views in this period</div>
            <div className="empty-state-desc">
              Share your video to start getting analytics data.
            </div>
          </div>
        </div>
      )}

      {hasViews && data.daily.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="card-header">
            <h3 className="card-title" style={{ margin: 0 }}>Views Over Time</h3>
            <span className="card-subtitle">{RANGE_SUBTITLES[range]}</span>
          </div>
          <CssBarChart daily={data.daily} />
        </div>
      )}

      {hasViews && data.heatmap && data.heatmap.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="card-header">
            <h3 className="card-title" style={{ margin: 0 }}>Viewer Retention</h3>
            <span className="card-subtitle">Avg: {Math.round(data.heatmap.reduce((sum, s) => sum + s.intensity, 0) / data.heatmap.length * 100)}%</span>
          </div>
          <div className="heatmap-bar-container">
            {Array.from({ length: 50 }, (_, i) => {
              const seg = data.heatmap!.find((s) => s.segment === i);
              const intensity = seg ? seg.intensity : 0;
              return (
                <div
                  key={i}
                  className="heatmap-segment"
                  data-tooltip={`${i * 2}%-${(i + 1) * 2}%: ${seg ? seg.watchCount : 0} views`}
                  style={{ opacity: Math.max(intensity, 0.08) }}
                />
              );
            })}
          </div>
          <div className="heatmap-labels">
            <span className="heatmap-label">0%</span>
            <span className="heatmap-label">50%</span>
            <span className="heatmap-label">100%</span>
          </div>
        </div>
      )}

      {hasViews && (
        <div className="card" style={{ marginBottom: 16 }}>
          <h3 className="card-title">Completion Funnel</h3>
          {[
            { label: "25%", value: data.milestones.reached25 },
            { label: "50%", value: data.milestones.reached50 },
            { label: "75%", value: data.milestones.reached75 },
            { label: "100%", value: data.milestones.reached100 },
          ].map((m) => {
            const pct =
              data.summary.totalViews > 0
                ? (m.value / data.summary.totalViews) * 100
                : 0;
            return (
              <div
                key={m.label}
                className="referrer-row"
              >
                <span className="referrer-label" style={{ width: 40, textAlign: "right" }}>
                  {m.label}
                </span>
                <div className="referrer-bar-track">
                  <div
                    className="referrer-bar-fill"
                    style={{
                      width: `${pct}%`,
                      minWidth: m.value > 0 ? 2 : 0,
                    }}
                  />
                </div>
                <span
                  style={{
                    color: "var(--color-text)",
                    fontSize: 13,
                    fontWeight: 600,
                    width: 50,
                    textAlign: "right",
                    flexShrink: 0,
                  }}
                >
                  {m.value}
                </span>
                <span className="referrer-pct">{pct.toFixed(0)}%</span>
              </div>
            );
          })}
        </div>
      )}

      {hasViews && data.referrers.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <h3 className="card-title">Top Referrers</h3>
          {data.referrers.map((r) => (
            <div key={r.source} className="referrer-row">
              <span className="referrer-label">{r.source}</span>
              <div className="referrer-bar-track">
                <div
                  className="referrer-bar-fill"
                  style={{
                    width: `${r.percentage}%`,
                    minWidth: r.count > 0 ? 2 : 0,
                  }}
                />
              </div>
              <span className="referrer-pct">
                {r.percentage.toFixed(0)}%
              </span>
            </div>
          ))}
        </div>
      )}

      {hasViews && data.viewers.length > 0 && (
        <ViewerTable
          viewers={data.viewers}
          sortColumn={sortColumn}
          sortDirection={sortDirection}
          visibleCount={visibleViewerCount}
          onSort={onSort}
          onShowMore={onShowMore}
          sortIndicator={sortIndicator}
        />
      )}

      {hasViews &&
        (data.browsers.length > 0 || data.devices.length > 0) && (
          <div className="card" style={{ marginBottom: 16 }}>
            <h3 className="card-title">Devices &amp; Browsers</h3>
            <div className="breakdown-grid">
              {data.browsers.length > 0 && (
                <div>
                  <h4 className="breakdown-title">Browsers</h4>
                  {data.browsers.map((b) => (
                    <div key={b.name} className="breakdown-row">
                      <span className="breakdown-name">{b.name}</span>
                      <span className="breakdown-pct">
                        {b.percentage.toFixed(0)}%
                      </span>
                    </div>
                  ))}
                </div>
              )}
              {data.devices.length > 0 && (
                <div>
                  <h4 className="breakdown-title">Devices</h4>
                  {data.devices.map((d) => (
                    <div key={d.name} className="breakdown-row">
                      <span className="breakdown-name">{d.name}</span>
                      <span className="breakdown-pct">
                        {d.percentage.toFixed(0)}%
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
    </>
  );
}
