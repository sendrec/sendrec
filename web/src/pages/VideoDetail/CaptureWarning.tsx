interface CaptureWarningProps {
  /** Set by the probe job when a recording arrived with no moving picture. */
  warning?: string | null;
}

export function CaptureWarning({ warning }: CaptureWarningProps) {
  if (!warning) return null;

  return (
    <p role="alert" className="capture-warning" data-testid="capture-warning">
      {warning}
    </p>
  );
}
