import { useCallback, useEffect, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiFetch, getAccessToken } from "../../api/client";
import type { View, Range, SortColumn, SortDirection, AnalyticsData, DashboardData } from "./types";
import { RANGES, RANGE_LABELS } from "./types";
import { SkeletonLoading } from "./SkeletonLoading";
import { VideoAnalyticsView } from "./VideoAnalyticsView";
import { DashboardView } from "./DashboardView";

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
