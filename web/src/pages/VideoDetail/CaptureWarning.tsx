interface CaptureWarningProps {
  /** Set by the jobs that write the file when its video died before the audio. */
  warning?: string | null;
}

// role="note" rather than "alert": the page renders nothing until the video has
// loaded, so this text is present the moment it mounts, and several screen
// readers do not announce a live region that arrives already filled.
export function CaptureWarning({ warning }: CaptureWarningProps) {
  if (!warning) return null;

  return (
    <p role="note" className="capture-warning" data-testid="capture-warning">
      {warning}
    </p>
  );
}
