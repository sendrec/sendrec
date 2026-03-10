export type View = "video" | "dashboard";
export type Range = "7d" | "30d" | "90d" | "all";
export type SortColumn = "viewer" | "watchTime" | "completion" | "date" | "location";
export type SortDirection = "asc" | "desc";

export interface AnalyticsSummary {
  totalViews: number;
  uniqueViews: number;
  viewsToday: number;
  averageDailyViews: number;
  peakDay: string;
  peakDayViews: number;
  totalCtaClicks: number;
  ctaClickRate: number;
}

export interface DailyViews {
  date: string;
  views: number;
  uniqueViews: number;
}

export interface Milestones {
  reached25: number;
  reached50: number;
  reached75: number;
  reached100: number;
}

export interface Viewer {
  email: string;
  firstViewedAt: string;
  viewCount: number;
  completion: number;
  watchTimeSeconds: number;
  country: string;
  city: string;
}

export interface SegmentData {
  segment: number;
  watchCount: number;
  intensity: number;
}

export interface Trends {
  views: number | null;
  uniqueViews: number | null;
  avgWatchTime: number | null;
  completionRate: number | null;
}

export interface Referrer {
  source: string;
  count: number;
  percentage: number;
}

export interface BrowserStat {
  name: string;
  percentage: number;
}

export interface DeviceStat {
  name: string;
  percentage: number;
}

export interface AnalyticsData {
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

export interface DashboardSummary {
  totalViews: number;
  uniqueViews: number;
  avgDailyViews: number;
  totalVideos: number;
  totalWatchTimeSeconds: number;
  avgCompletion: number;
}

export interface DashboardTopVideo {
  id: string;
  title: string;
  views: number;
  uniqueViews: number;
  thumbnailUrl: string;
  completion: number;
}

export interface DashboardData {
  summary: DashboardSummary;
  daily: DailyViews[];
  topVideos: DashboardTopVideo[];
}

export const RANGES: Range[] = ["7d", "30d", "90d", "all"];

export const RANGE_LABELS: Record<Range, string> = {
  "7d": "7d",
  "30d": "30d",
  "90d": "90d",
  all: "All",
};

export const RANGE_SUBTITLES: Record<Range, string> = {
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  "90d": "Last 90 days",
  all: "All time",
};

export function formatWatchTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (remainingMinutes === 0) return `${hours}h`;
  return `${hours}h ${remainingMinutes}m`;
}

export function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB");
}

export function formatTrend(value: number | null): string {
  if (value === null) return "";
  const sign = value >= 0 ? "+" : "";
  return `${sign}${value.toFixed(0)}%`;
}

export function trendDirection(value: number | null): "up" | "down" | null {
  if (value === null || value === 0) return null;
  return value > 0 ? "up" : "down";
}

export function sortViewers(
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
