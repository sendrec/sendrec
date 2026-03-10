export function SkeletonLoading() {
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
