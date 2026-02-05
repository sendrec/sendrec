import { Routes, Route, Navigate } from "react-router-dom";

export function App() {
  return (
    <Routes>
      <Route path="/" element={<div>SendRec — coming soon</div>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
