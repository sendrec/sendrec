import type { Viewer, SortColumn, SortDirection } from "./types";
import { sortViewers, formatWatchTime, formatTimestamp } from "./types";

export function ViewerTable({
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
