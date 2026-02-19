import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { apiFetch } from "../api/client";

interface DailyViews {
  date: string;
  views: number;
  uniqueViews: number;
}

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

interface AnalyticsData {
  summary: AnalyticsSummary;
  daily: DailyViews[];
}

type Range = "7d" | "30d" | "all";

function formatPeakDate(isoDate: string): string {
  if (!isoDate) return "";
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatChartDate(isoDate: string): string {
  const date = new Date(isoDate + "T00:00:00");
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

export function Analytics() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [range, setRange] = useState<Range>("7d");
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<unknown>(null);

  const fetchAnalytics = useCallback(async (selectedRange: Range) => {
    setLoading(true);
    setError(false);
    try {
      const result = await apiFetch<AnalyticsData>(`/api/videos/${id}/analytics?range=${selectedRange}`);
      if (result) {
        setData(result);
      } else {
        setError(true);
      }
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchAnalytics(range);
  }, [range, fetchAnalytics]);

  useEffect(() => {
    if (!data || data.summary.totalViews === 0 || !canvasRef.current) return;

    let destroyed = false;

    async function renderChart() {
      const chartModule = await import("chart.js/auto");
      const Chart = chartModule.default;

      if (destroyed || !canvasRef.current) return;

      if (chartRef.current) {
        (chartRef.current as { destroy: () => void }).destroy();
      }

      chartRef.current = new Chart(canvasRef.current, {
        type: "bar",
        data: {
          labels: data!.daily.map((d) => formatChartDate(d.date)),
          datasets: [
            {
              label: "Views",
              data: data!.daily.map((d) => d.views),
              backgroundColor: "var(--color-accent)",
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            tooltip: {
              callbacks: {
                afterLabel: (context) => {
                  const dayData = data!.daily[context.dataIndex];
                  return `Unique: ${dayData.uniqueViews}`;
                },
              },
            },
          },
          scales: {
            y: {
              beginAtZero: true,
              ticks: {
                stepSize: 1,
              },
            },
          },
        },
      });
    }

    renderChart();

    return () => {
      destroyed = true;
      if (chartRef.current) {
        (chartRef.current as { destroy: () => void }).destroy();
        chartRef.current = null;
      }
    };
  }, [data]);

  function handleRangeChange(newRange: Range) {
    setRange(newRange);
  }

  if (loading) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-text-secondary)", fontSize: 16 }}>Loading...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="page-container page-container--centered">
        <p style={{ color: "var(--color-error)", fontSize: 16 }}>Failed to load analytics.</p>
      </div>
    );
  }

  const ranges: Range[] = ["7d", "30d", "all"];

  return (
    <div className="page-container">
      <Link
        to="/library"
        style={{
          color: "var(--color-text-secondary)",
          textDecoration: "none",
          fontSize: 14,
          marginBottom: 16,
          display: "inline-block",
        }}
      >
        &larr; Library
      </Link>

      <div className="analytics-header">
        <h1 style={{ color: "var(--color-text)", fontSize: 24, margin: 0 }}>
          Analytics
        </h1>
        <div style={{ display: "flex", gap: 4 }}>
          {ranges.map((r) => (
            <button
              key={r}
              onClick={() => handleRangeChange(r)}
              style={{
                background: range === r ? "var(--color-accent)" : "transparent",
                color: range === r ? "#fff" : "var(--color-text-secondary)",
                border: `1px solid ${range === r ? "var(--color-accent)" : "var(--color-border)"}`,
                borderRadius: 4,
                padding: "6px 12px",
                fontSize: 13,
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              {r === "all" ? "All" : r}
            </button>
          ))}
        </div>
      </div>

      <div className="analytics-stats">
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Total Views</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.totalViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Unique Views</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.uniqueViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Avg / Day</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.averageDailyViews}
          </p>
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>Peak Day</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.peakDayViews}
          </p>
          {data.summary.peakDay && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 12, margin: "4px 0 0" }}>
              {formatPeakDate(data.summary.peakDay)}
            </p>
          )}
        </div>
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            textAlign: "center",
          }}
        >
          <p style={{ color: "var(--color-text-secondary)", fontSize: 13, margin: "0 0 4px" }}>CTA Clicks</p>
          <p style={{ color: "var(--color-text)", fontSize: 24, fontWeight: 700, margin: 0 }}>
            {data.summary.totalCtaClicks}
          </p>
          {data.summary.totalViews > 0 && (
            <p style={{ color: "var(--color-text-secondary)", fontSize: 12, margin: "4px 0 0" }}>
              {(data.summary.ctaClickRate * 100).toFixed(1)}% click rate
            </p>
          )}
        </div>
      </div>

      {data.summary.totalViews === 0 ? (
        <p style={{ color: "var(--color-text-secondary)", fontSize: 14, textAlign: "center", marginTop: 32 }}>
          No views in this period.
        </p>
      ) : (
        <div
          style={{
            background: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            padding: 16,
            height: 300,
          }}
        >
          <canvas ref={canvasRef} />
        </div>
      )}
    </div>
  );
}
