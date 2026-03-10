import { Link } from "react-router-dom";
import type { DashboardData, Range } from "./types";
import { RANGE_SUBTITLES, formatWatchTime } from "./types";
import { StatCard } from "./StatCard";
import { CssBarChart } from "./CssBarChart";

export function DashboardView({ data, range }: { data: DashboardData; range: Range }) {
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
          value={`${Math.round(data.summary.avgCompletion)}%`}
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
