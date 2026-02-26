import { useCallback, useEffect, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiFetch, getAccessToken } from "../api/client";

type View = "video" | "dashboard";
type Range = "7d" | "30d" | "90d" | "all";
type SortColumn = "viewer" | "watchTime" | "completion" | "date" | "location";
type SortDirection = "asc" | "desc";

interface AnalyticsSummary {
  totalViews: number;
  uniqueViews: number;
  viewsToday: number;
  averageDailyViews: number;
  peakDay: string;
  peakDayViews: number;
  totalCtaClicks: number;
  ctaClickRate: number;
}

interface DailyViews {
  date: string;
  views: number;
  uniqueViews: number;
}

interface Milestones {
  reached25: number;
  reached50: number;
  reached75: number;
  reached100: number;
}

interface Viewer {
  email: string;
  firstViewedAt: string;
  viewCount: number;
  completion: number;
  watchTimeSeconds: number;
  country: string;
  city: string;
}

interface SegmentData {
  segment: number;
  watchCount: number;
  intensity: number;
}

interface Trends {
  views: number | null;
  uniqueViews: number | null;
  avgWatchTime: number | null;
  completionRate: number | null;
}

interface Referrer {
  source: string;
  count: number;
  percentage: number;
}

interface BrowserStat {
  name: string;
  percentage: number;
}

interface DeviceStat {
  name: string;
  percentage: number;
}

interface AnalyticsData {
  summary: AnalyticsSummary;
  daily: DailyViews[];
  milestones: Milestones;
  viewers: Viewer[];
  heatmap: SegmentData[] | null;
  trends?: Trends | null;
  referrers: Referrer[];
  browsers: BrowserStat[];
  devices: DeviceStat[];
}

interface DashboardSummary {
  totalViews: number;
  uniqueViews: number;
  avgDailyViews: number;
  totalVideos: number;
  totalWatchTimeSeconds: number;
  avgCompletion: number;
}

interface DashboardTopVideo {
  id: string;
  title: string;
  views: number;
  uniqueViews: number;
  thumbnailUrl: string;
  completion: number;
}

interface DashboardData {
  summary: DashboardSummary;
  daily: DailyViews[];
  topVideos: DashboardTopVideo[];
}

function formatDate(isoDate: string): string {
  if (!isoDate) return "";
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
  });
}

function formatChartDate(isoDate: string): string {
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
  });
}

function formatWatchTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (remainingMinutes === 0) return `${hours}h`;
  return `${hours}h ${remainingMinutes}m`;
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB");
}

function formatTrend(value: number | null): string {
  if (value === null) return "";
  const sign = value >= 0 ? "+" : "";
  return `${sign}${value.toFixed(0)}%`;
}

function trendDirection(value: number | null): "up" | "down" | null {
  if (value === null || value === 0) return null;
  return value > 0 ? "up" : "down";
}

const RANGES: Range[] = ["7d", "30d", "90d", "all"];

const RANGE_LABELS: Record<Range, string> = {
  "7d": "7d",
  "30d": "30d",
  "90d": "90d",
  all: "All",
};

const RANGE_SUBTITLES: Record<Range, string> = {
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  "90d": "Last 90 days",
  all: "All time",
};

function sortViewers(
  viewers: Viewer[],
  column: SortColumn,
  direction: SortDirection,
): Viewer[] {
  return [...viewers].sort((a, b) => {
    let comparison = 0;
    switch (column) {
      case "viewer":
        comparison = a.email.localeCompare(b.email);
        break;
      case "watchTime":
        comparison = a.watchTimeSeconds - b.watchTimeSeconds;
        break;
      case "completion":
        comparison = a.completion - b.completion;
        break;
      case "date":
        comparison =
          new Date(a.firstViewedAt).getTime() -
          new Date(b.firstViewedAt).getTime();
        break;
      case "location": {
        const locA = [a.city, a.country].filter(Boolean).join(", ");
        const locB = [b.city, b.country].filter(Boolean).join(", ");
        comparison = locA.localeCompare(locB);
        break;
      }
    }
    return direction === "asc" ? comparison : -comparison;
  });
}

function SkeletonLoading() {
  return (
    <div className="page-container">
      <div className="analytics-header">
        <div className="skeleton" style={{ width: 120, height: 28 }} />
        <div style={{ display: "flex", gap: 8 }}>
          <div className="skeleton" style={{ width: 60, height: 28 }} />
          <div className="skeleton" style={{ width: 60, height: 28 }} />
        </div>
      </div>
      <div className="analytics-stats">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="skeleton skeleton-stat" />
        ))}
      </div>
      <div className="skeleton skeleton-chart" style={{ marginBottom: 16 }} />
      <div className="skeleton skeleton-chart" />
    </div>
  );
}

function CssBarChart({ daily }: { daily: DailyViews[] }) {
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

export function Analytics() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [view, setView] = useState<View>(id ? "video" : "dashboard");
  const [range, setRange] = useState<Range>("7d");

  const [videoData, setVideoData] = useState<AnalyticsData | null>(null);
  const [dashboardData, setDashboardData] = useState<DashboardData | null>(
    null,
  );

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const [sortColumn, setSortColumn] = useState<SortColumn>("date");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const [visibleViewerCount, setVisibleViewerCount] = useState(7);

  useEffect(() => {
    setView(id ? "video" : "dashboard");
  }, [id]);

  const fetchData = useCallback(
    async (currentView: View, currentRange: Range) => {
      setLoading(true);
      setError(false);
      try {
        if (currentView === "video" && id) {
          const result = await apiFetch<AnalyticsData>(
            `/api/videos/${id}/analytics?range=${currentRange}`,
          );
          if (result) {
            setVideoData(result);
          } else {
            setError(true);
          }
        } else {
          const result = await apiFetch<DashboardData>(
            `/api/analytics/dashboard?range=${currentRange}`,
          );
          if (result) {
            setDashboardData(result);
          } else {
            setError(true);
          }
        }
      } catch {
        setError(true);
      } finally {
        setLoading(false);
      }
    },
    [id],
  );

  useEffect(() => {
    fetchData(view, range);
  }, [view, range, fetchData]);

  function handleViewToggle(newView: View) {
    if (newView === view) return;
    if (newView === "video" && !id) return;

    setView(newView);
    setVisibleViewerCount(7);
    setSortColumn("date");
    setSortDirection("desc");

    if (newView === "dashboard" && id) {
      navigate("/analytics");
    }
  }

  function handleSort(column: SortColumn) {
    if (sortColumn === column) {
      setSortDirection((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(column);
      setSortDirection("desc");
    }
  }

  function sortIndicator(column: SortColumn): string {
    if (sortColumn !== column) return "";
    return sortDirection === "asc" ? " \u2191" : " \u2193";
  }

  async function handleExport() {
    const url =
      view === "video"
        ? `/api/videos/${id}/analytics/export?range=${range}`
        : `/api/analytics/dashboard/export?range=${range}`;
    const token = getAccessToken();
    try {
      const res = await fetch(url, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) return;
      const blob = await res.blob();
      const anchor = document.createElement("a");
      anchor.href = URL.createObjectURL(blob);
      anchor.download =
        view === "video"
          ? `analytics-${id}.csv`
          : "dashboard-analytics.csv";
      anchor.click();
      setTimeout(() => URL.revokeObjectURL(anchor.href), 1000);
    } catch {
      // Export failed silently
    }
  }

  if (loading) {
    return <SkeletonLoading />;
  }

  if (error) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-error)", fontSize: 16 }}>
          Failed to load analytics.
        </p>
      </div>
    );
  }

  return (
    <div className="page-container">
      {view === "video" && id && (
        <Link to={`/videos/${id}`} className="back-link">
          &larr; Back
        </Link>
      )}

      <div className="analytics-header">
        <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <h1
            style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}
          >
            Analytics
          </h1>
          <div className="analytics-toggle">
            <button
              className={`analytics-toggle-btn${view === "video" ? " analytics-toggle-btn--active" : ""}`}
              onClick={() => handleViewToggle("video")}
              disabled={!id && view === "dashboard"}
            >
              Video
            </button>
            <button
              className={`analytics-toggle-btn${view === "dashboard" ? " analytics-toggle-btn--active" : ""}`}
              onClick={() => handleViewToggle("dashboard")}
            >
              Dashboard
            </button>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <div className="range-pills">
            {RANGES.map((r) => (
              <button
                key={r}
                className={`range-pill${range === r ? " range-pill--active" : ""}`}
                onClick={() => setRange(r)}
              >
                {RANGE_LABELS[r]}
              </button>
            ))}
          </div>
          <button className="btn-export" onClick={handleExport}>
            Export CSV
          </button>
        </div>
      </div>

      {view === "video" && videoData ? (
        <VideoAnalyticsView
          data={videoData}
          range={range}
          sortColumn={sortColumn}
          sortDirection={sortDirection}
          visibleViewerCount={visibleViewerCount}
          onSort={handleSort}
          onShowMore={() =>
            setVisibleViewerCount((prev) => prev + 10)
          }
          sortIndicator={sortIndicator}
        />
      ) : view === "dashboard" && dashboardData ? (
        <DashboardView data={dashboardData} range={range} />
      ) : (
        <div className="page-container page-container--centered">
          <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>
            No data available.
          </p>
        </div>
      )}
    </div>
  );
}

function VideoAnalyticsView({
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
          sub={data.summary.peakDay ? formatDate(data.summary.peakDay) : undefined}
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

function DashboardView({ data, range }: { data: DashboardData; range: Range }) {
  const hasViews = data.summary.totalViews > 0;

  return (
    <>
      <div className="analytics-stats">
        <StatCard label="Total Views" value={data.summary.totalViews} />
        <StatCard
          label="Unique Viewers"
          value={data.summary.uniqueViews}
        />
        <StatCard label="Avg / Day" value={data.summary.avgDailyViews} />
        <StatCard label="Total Videos" value={data.summary.totalVideos} />
        <StatCard
          label="Watch Time"
          value={formatWatchTime(data.summary.totalWatchTimeSeconds)}
        />
        <StatCard
          label="Avg Completion"
          value={`${data.summary.avgCompletion}%`}
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
            <div className="empty-state-title">
              No analytics yet
            </div>
            <div className="empty-state-desc">
              Views will appear here once your videos are watched.
            </div>
          </div>
        </div>
      )}

      {hasViews && data.topVideos.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="card-header">
            <h3 className="card-title" style={{ margin: 0 }}>Top Videos</h3>
            <span className="card-subtitle">{RANGE_SUBTITLES[range]}</span>
          </div>
          {data.topVideos.map((video, index) => (
            <Link
              key={video.id}
              to={`/videos/${video.id}/analytics`}
              className="top-video-row"
            >
              <span className="top-video-rank">{index + 1}</span>
              <div className="top-video-thumb">
                {video.thumbnailUrl && (
                  <img src={video.thumbnailUrl} alt="" />
                )}
              </div>
              <div className="top-video-info">
                <div className="top-video-title">{video.title}</div>
              </div>
              <div className="top-video-stats">
                <div className="top-video-stat">
                  <div className="top-video-stat-value">{video.views}</div>
                  <div className="top-video-stat-label">views</div>
                </div>
                <div className="top-video-stat">
                  <div className="top-video-stat-value">
                    {video.uniqueViews}
                  </div>
                  <div className="top-video-stat-label">unique</div>
                </div>
                <div className="top-video-stat">
                  <div className="top-video-stat-value">
                    {video.completion}%
                  </div>
                  <div className="top-video-stat-label">completion</div>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}

      {hasViews && data.daily.length > 0 && (
        <div className="card">
          <div className="card-header">
            <h3 className="card-title" style={{ margin: 0 }}>Total Views Over Time</h3>
            <span className="card-subtitle">{RANGE_SUBTITLES[range]}</span>
          </div>
          <CssBarChart daily={data.daily} />
        </div>
      )}
    </>
  );
}

function StatCard({
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

function ViewerTable({
  viewers,
  sortColumn,
  sortDirection,
  visibleCount,
  onSort,
  onShowMore,
  sortIndicator,
}: {
  viewers: Viewer[];
  sortColumn: SortColumn;
  sortDirection: SortDirection;
  visibleCount: number;
  onSort: (column: SortColumn) => void;
  onShowMore: () => void;
  sortIndicator: (column: SortColumn) => string;
}) {
  const sorted = sortViewers(viewers, sortColumn, sortDirection);
  const visible = sorted.slice(0, visibleCount);
  const remaining = sorted.length - visibleCount;

  return (
    <div className="card" style={{ marginBottom: 16 }}>
      <h3 className="card-title">Viewer Activity</h3>
      <table className="viewers-table">
        <thead>
          <tr>
            <th onClick={() => onSort("viewer")}>
              Viewer{sortIndicator("viewer")}
            </th>
            <th data-align="right" onClick={() => onSort("watchTime")}>
              Watch Time{sortIndicator("watchTime")}
            </th>
            <th data-align="right" onClick={() => onSort("completion")}>
              Completion{sortIndicator("completion")}
            </th>
            <th onClick={() => onSort("date")}>
              Date{sortIndicator("date")}
            </th>
            <th onClick={() => onSort("location")}>
              Location{sortIndicator("location")}
            </th>
          </tr>
        </thead>
        <tbody>
          {visible.map((v, i) => {
            const location = [v.city, v.country]
              .filter(Boolean)
              .join(", ");
            return (
              <tr key={`${v.email}-${i}`}>
                <td>
                  {v.email ? (
                    v.email
                  ) : (
                    <span className="viewer-anonymous">
                      Anonymous
                    </span>
                  )}
                </td>
                <td data-align="right">
                  {formatWatchTime(v.watchTimeSeconds)}
                </td>
                <td data-align="right">
                  {v.completion}%
                  <span className="viewer-completion-bar">
                    <span
                      className="viewer-completion-fill"
                      style={{ width: `${v.completion}%` }}
                    />
                  </span>
                </td>
                <td>{formatTimestamp(v.firstViewedAt)}</td>
                <td>{location || "-"}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {remaining > 0 && (
        <button className="show-more-btn" onClick={onShowMore}>
          Show {Math.min(remaining, 10)} more viewer
          {Math.min(remaining, 10) !== 1 ? "s" : ""}
        </button>
      )}
    </div>
  );
}
