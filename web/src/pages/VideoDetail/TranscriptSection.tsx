import { useRef, useState } from "react";
import { apiFetch } from "../../api/client";
import { useToast } from "../../hooks/useToast";
import { Toast } from "../../components/Toast";
import { DocumentModal } from "../../components/DocumentModal";
import { TRANSCRIPTION_LANGUAGES } from "../../constants/languages";
import type { Video } from "../../types/video";
import type { TranscriptSegment } from "../../types/transcript";
import { LimitsResponse } from "../../types/limits";
import { formatDuration } from "../../utils/format";

interface TranscriptSectionProps {
  video: Video;
  limits: LimitsResponse | null;
  isViewer: boolean;
  transcriptSegments: TranscriptSegment[];
  retranscribeLanguage: string;
  onRetranscribeLanguageChange: (language: string) => void;
  onVideoUpdate: (updater: (prev: Video | null) => Video | null) => void;
  onTranscriptClear: () => void;
  onTranscriptSegmentsUpdate: (segments: TranscriptSegment[]) => void;
}

export function TranscriptSection({
  video,
  limits,
  isViewer,
  transcriptSegments,
  retranscribeLanguage,
  onRetranscribeLanguageChange,
  onVideoUpdate,
  onTranscriptClear,
  onTranscriptSegmentsUpdate,
}: TranscriptSectionProps) {
  const toast = useToast();
  const [showDocumentModal, setShowDocumentModal] = useState(false);
  const [documentContent, setDocumentContent] = useState<string | null>(null);
  const [uploadingTranscript, setUploadingTranscript] = useState(false);
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const currentVideoId = useRef(video.id);
  currentVideoId.current = video.id;

  async function retranscribe() {
    const body =
      retranscribeLanguage !== "auto"
        ? { language: retranscribeLanguage }
        : undefined;
    await apiFetch(`/api/videos/${video.id}/retranscribe`, {
      method: "POST",
      ...(body && { body: JSON.stringify(body) }),
    });
    onVideoUpdate((prev) =>
      prev ? { ...prev, transcriptStatus: "pending" } : prev,
    );
    onTranscriptClear();
    toast.show("Transcription queued");
  }

  async function uploadTranscript(file: File) {
    const uploadVideoId = video.id;
    const formData = new FormData();
    formData.append("file", file);
    setUploadingTranscript(true);
    try {
      const data = await apiFetch<{ segments: TranscriptSegment[] }>(
        `/api/videos/${uploadVideoId}/transcript`,
        { method: "POST", body: formData },
      );
      // The user may have navigated to another video while the upload was in
      // flight; only apply the result if this is still the shown video.
      // The user may have navigated to another video while the upload was in
      // flight; only apply the result if this is still the shown video.
      if (currentVideoId.current !== uploadVideoId) return;
      onVideoUpdate((prev) =>
        prev ? { ...prev, transcriptStatus: "ready" } : prev,
      );
      onTranscriptSegmentsUpdate(data?.segments ?? []);
      toast.show("Transcript uploaded");
    } catch (err) {
      if (currentVideoId.current !== uploadVideoId) return;
      toast.show(
        err instanceof Error ? err.message : "Failed to upload transcript",
      );
    } finally {
      if (currentVideoId.current === uploadVideoId) {
        setUploadingTranscript(false);
      }
    }
  }

  function handleUploadInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (file) {
      void uploadTranscript(file);
    }
  }

  async function summarize() {
    await apiFetch(`/api/videos/${video.id}/summarize`, { method: "POST" });
    onVideoUpdate((prev) =>
      prev ? { ...prev, summaryStatus: "pending" } : prev,
    );
    toast.show("Summary queued");
  }

  async function viewDocument() {
    const data = await apiFetch<{ document?: string }>(
      `/api/watch/${video.shareToken}`,
    );
    if (data?.document) {
      setDocumentContent(data.document);
      setShowDocumentModal(true);
    }
  }

  async function generateDocument() {
    await apiFetch(`/api/videos/${video.id}/generate-document`, {
      method: "POST",
    });
    onVideoUpdate((prev) =>
      prev ? { ...prev, documentStatus: "pending" } : prev,
    );
    toast.show("Document generation queued");
  }

  return (
    <>
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">AI</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Transcript</span>
          <div className="detail-setting-value">
            <span>
              {video.transcriptStatus === "none" && "Not started"}
              {video.transcriptStatus === "pending" && "Pending..."}
              {video.transcriptStatus === "processing" && "Transcribing..."}
              {video.transcriptStatus === "ready" && "Ready"}
              {video.transcriptStatus === "no_audio" && "No audio"}
              {video.transcriptStatus === "failed" && "Failed"}
            </span>
            {!isViewer &&
              (video.transcriptStatus === "none" ||
                video.transcriptStatus === "ready" ||
                video.transcriptStatus === "failed" ||
                video.transcriptStatus === "no_audio") && (
                <>
                  {limits?.transcriptionEnabled && (
                    <select
                      aria-label="Transcription language"
                      value={retranscribeLanguage}
                      onChange={(e) =>
                        onRetranscribeLanguageChange(e.target.value)
                      }
                      className="detail-select"
                    >
                      {TRANSCRIPTION_LANGUAGES.map((lang) => (
                        <option key={lang.code} value={lang.code}>
                          {lang.name}
                        </option>
                      ))}
                    </select>
                  )}
                  <button onClick={retranscribe} className="detail-btn">
                    {video.transcriptStatus === "none"
                      ? "Transcribe"
                      : video.transcriptStatus === "ready"
                        ? "Redo transcript"
                        : "Retry transcript"}
                  </button>
                  <button
                    onClick={() => uploadInputRef.current?.click()}
                    disabled={uploadingTranscript}
                    className="detail-btn"
                    style={{
                      opacity: uploadingTranscript ? 0.5 : undefined,
                    }}
                  >
                    {uploadingTranscript ? "Uploading..." : "Upload transcript"}
                  </button>
                  <input
                    ref={uploadInputRef}
                    type="file"
                    accept=".vtt,text/vtt"
                    data-testid="transcript-upload-input"
                    onChange={handleUploadInputChange}
                    disabled={uploadingTranscript}
                    style={{ display: "none" }}
                  />
                </>
              )}
          </div>
        </div>

        {video.transcriptStatus === "ready" &&
          transcriptSegments.length > 0 && (
            <div className="transcript-segments">
              {transcriptSegments.map((seg, i) => (
                <div key={i} className="transcript-segment">
                  <span className="transcript-segment-time">
                    {formatDuration(seg.start)}
                  </span>
                  <span className="transcript-segment-text">
                    {seg.speaker && (
                      <span className="transcript-segment-speaker">
                        {seg.speaker}
                      </span>
                    )}
                    {seg.text}
                  </span>
                </div>
              ))}
            </div>
          )}

        {!isViewer && limits?.aiEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Summary</span>
            <div className="detail-setting-value">
              <span>
                {video.summaryStatus === "none" && "Not started"}
                {video.summaryStatus === "pending" && "Pending..."}
                {video.summaryStatus === "processing" && "Summarizing..."}
                {video.summaryStatus === "ready" && "Ready"}
                {video.summaryStatus === "too_short" && "Transcript too short"}
                {video.summaryStatus === "failed" && "Failed"}
              </span>
              <button
                onClick={summarize}
                disabled={
                  video.transcriptStatus !== "ready" ||
                  video.summaryStatus === "pending" ||
                  video.summaryStatus === "processing"
                }
                className="detail-btn"
                style={{
                  opacity:
                    video.transcriptStatus !== "ready" ||
                    video.summaryStatus === "pending" ||
                    video.summaryStatus === "processing"
                      ? 0.5
                      : undefined,
                }}
              >
                {video.summaryStatus === "ready"
                  ? "Re-summarize"
                  : "Summarize"}
              </button>
            </div>
          </div>
        )}

        {!isViewer && limits?.aiEnabled && (
          <div className="detail-setting-row">
            <span className="detail-setting-label">Document</span>
            <div className="detail-setting-value">
              <span>
                {video.documentStatus === "none" && "Not generated"}
                {video.documentStatus === "pending" && "Pending..."}
                {video.documentStatus === "processing" && "Generating..."}
                {video.documentStatus === "ready" && "Ready"}
                {video.documentStatus === "too_short" &&
                  "Transcript too short"}
                {video.documentStatus === "failed" && "Failed"}
              </span>
              {video.documentStatus === "ready" ? (
                <>
                  <button
                    onClick={viewDocument}
                    className="detail-btn detail-btn--accent"
                  >
                    View
                  </button>
                  <button onClick={generateDocument} className="detail-btn">
                    Regenerate
                  </button>
                </>
              ) : (
                <button
                  onClick={generateDocument}
                  disabled={
                    video.transcriptStatus !== "ready" ||
                    video.documentStatus === "pending" ||
                    video.documentStatus === "processing"
                  }
                  className="detail-btn"
                  style={{
                    opacity:
                      video.transcriptStatus !== "ready" ||
                      video.documentStatus === "pending" ||
                      video.documentStatus === "processing"
                        ? 0.5
                        : undefined,
                  }}
                >
                  Generate
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Document Modal */}
      {showDocumentModal && documentContent && (
        <DocumentModal
          document={documentContent}
          onClose={() => {
            setShowDocumentModal(false);
            setDocumentContent(null);
          }}
        />
      )}

      <Toast message={toast.message} />
    </>
  );
}
