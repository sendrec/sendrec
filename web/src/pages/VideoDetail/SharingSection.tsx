import { useState } from "react";
import { apiFetch } from "../../api/client";
import { useToast } from "../../hooks/useToast";
import { Toast } from "../../components/Toast";
import { PromptDialog } from "../../components/PromptDialog";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import type { Video } from "../../types/video";
import { expiryLabel } from "../../utils/format";
import { copyToClipboard } from "../../utils/clipboard";
import { LimitsResponse } from "../../types/limits";

interface VideoBranding {
  companyName: string | null;
  colorBackground: string | null;
  colorSurface: string | null;
  colorText: string | null;
  colorAccent: string | null;
  footerText: string | null;
}

interface SharingSectionProps {
  video: Video;
  limits: LimitsResponse | null;
  isViewer: boolean;
  onVideoUpdate: (updater: (prev: Video | null) => Video | null) => void;
  onRefetchVideo: () => Promise<void>;
}

export function SharingSection({
  video,
  limits,
  isViewer,
  onVideoUpdate,
  onRefetchVideo,
}: SharingSectionProps) {
  const toast = useToast();

  const [uploadingThumbnail, setUploadingThumbnail] = useState(false);
  const [brandingOpen, setBrandingOpen] = useState(false);
  const [videoBranding, setVideoBranding] = useState<VideoBranding>({
    companyName: null,
    colorBackground: null,
    colorSurface: null,
    colorText: null,
    colorAccent: null,
    footerText: null,
  });
  const [savingBranding, setSavingBranding] = useState(false);
  const [brandingMessage, setBrandingMessage] = useState("");
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(
    null,
  );
  const [promptDialog, setPromptDialog] = useState<{
    title: string;
    onSubmit: (value: string) => void;
    placeholder?: string;
    submitLabel?: string;
  } | null>(null);

  const expiry = expiryLabel(video.shareExpiresAt);
  const embedSnippet = `<iframe src="${window.location.origin}/embed/${video.shareToken}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;

  async function copyLink() {
    await copyToClipboard(video.shareUrl);
    toast.show("Link copied");
  }

  async function copyEmbed() {
    await copyToClipboard(embedSnippet);
    toast.show("Embed code copied");
  }

  async function toggleDownload() {
    const newValue = !video.downloadEnabled;
    await apiFetch(`/api/videos/${video.id}/download-enabled`, {
      method: "PUT",
      body: JSON.stringify({ downloadEnabled: newValue }),
    });
    onVideoUpdate((prev) =>
      prev ? { ...prev, downloadEnabled: newValue } : prev,
    );
  }

  async function toggleEmailGate() {
    const newValue = !video.emailGateEnabled;
    await apiFetch(`/api/videos/${video.id}/email-gate`, {
      method: "PUT",
      body: JSON.stringify({ enabled: newValue }),
    });
    onVideoUpdate((prev) =>
      prev ? { ...prev, emailGateEnabled: newValue } : prev,
    );
  }

  async function toggleLinkExpiry() {
    const neverExpires = video.shareExpiresAt !== null;
    await apiFetch(`/api/videos/${video.id}/link-expiry`, {
      method: "PUT",
      body: JSON.stringify({ neverExpires }),
    });
    await onRefetchVideo();
  }

  async function extendVideo() {
    await apiFetch(`/api/videos/${video.id}/extend`, { method: "POST" });
    await onRefetchVideo();
    toast.show("Link extended");
  }

  function addPassword() {
    setPromptDialog({
      title: "Enter a password for this video:",
      placeholder: "Password",
      submitLabel: "Set password",
      onSubmit: async (password) => {
        setPromptDialog(null);
        await apiFetch(`/api/videos/${video.id}/password`, {
          method: "PUT",
          body: JSON.stringify({ password }),
        });
        onVideoUpdate((prev) =>
          prev ? { ...prev, hasPassword: true } : prev,
        );
      },
    });
  }

  function removePassword() {
    setConfirmDialog({
      message: "Remove the password from this video?",
      confirmLabel: "Remove",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await apiFetch(`/api/videos/${video.id}/password`, {
          method: "PUT",
          body: JSON.stringify({ password: "" }),
        });
        onVideoUpdate((prev) =>
          prev ? { ...prev, hasPassword: false } : prev,
        );
      },
    });
  }

  async function changeCommentMode(mode: string) {
    try {
      await apiFetch(`/api/videos/${video.id}/comment-mode`, {
        method: "PUT",
        body: JSON.stringify({ commentMode: mode }),
      });
      onVideoUpdate((prev) =>
        prev ? { ...prev, commentMode: mode } : prev,
      );
    } catch {
      // select stays at previous value
    }
  }

  async function uploadThumbnail(file: File) {
    if (file.size > 2 * 1024 * 1024) return;
    const validTypes = ["image/jpeg", "image/png", "image/webp"];
    if (!validTypes.includes(file.type)) return;
    setUploadingThumbnail(true);
    try {
      const result = await apiFetch<{ uploadUrl: string }>(
        `/api/videos/${video.id}/thumbnail`,
        {
          method: "POST",
          body: JSON.stringify({
            contentType: file.type,
            contentLength: file.size,
          }),
        },
      );
      if (!result) return;
      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!uploadResp.ok) return;
      await onRefetchVideo();
      toast.show("Thumbnail updated");
    } finally {
      setUploadingThumbnail(false);
    }
  }

  async function resetThumbnail() {
    setUploadingThumbnail(true);
    try {
      await apiFetch(`/api/videos/${video.id}/thumbnail`, {
        method: "DELETE",
      });
      await onRefetchVideo();
      toast.show("Thumbnail reset");
    } finally {
      setUploadingThumbnail(false);
    }
  }

  async function changeNotification(value: string) {
    const viewNotification = value === "" ? null : value;
    try {
      await apiFetch(`/api/videos/${video.id}/notifications`, {
        method: "PUT",
        body: JSON.stringify({ viewNotification }),
      });
      onVideoUpdate((prev) =>
        prev ? { ...prev, viewNotification } : prev,
      );
    } catch {
      // select stays at previous value
    }
  }

  async function openBranding() {
    setBrandingOpen(true);
    setBrandingMessage("");
    try {
      const data = await apiFetch<VideoBranding>(
        `/api/videos/${video.id}/branding`,
      );
      if (data) {
        setVideoBranding(data);
      } else {
        setVideoBranding({
          companyName: null,
          colorBackground: null,
          colorSurface: null,
          colorText: null,
          colorAccent: null,
          footerText: null,
        });
      }
    } catch {
      setVideoBranding({
        companyName: null,
        colorBackground: null,
        colorSurface: null,
        colorText: null,
        colorAccent: null,
        footerText: null,
      });
    }
  }

  async function saveBranding() {
    setSavingBranding(true);
    setBrandingMessage("");
    try {
      await apiFetch(`/api/videos/${video.id}/branding`, {
        method: "PUT",
        body: JSON.stringify({
          companyName: videoBranding.companyName || null,
          colorBackground: videoBranding.colorBackground || null,
          colorSurface: videoBranding.colorSurface || null,
          colorText: videoBranding.colorText || null,
          colorAccent: videoBranding.colorAccent || null,
          footerText: videoBranding.footerText || null,
        }),
      });
      setBrandingMessage("Saved");
      setTimeout(() => setBrandingOpen(false), 1000);
    } catch (err) {
      setBrandingMessage(
        err instanceof Error ? err.message : "Failed to save",
      );
    } finally {
      setSavingBranding(false);
    }
  }

  return (
    <>
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Share Settings</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Share link</span>
          {video.status === "processing" ? (
            <span
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 13,
              }}
            >
              Available once processing completes
            </span>
          ) : (
            <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
              <input
                type="text"
                readOnly
                value={video.shareUrl}
                aria-label="Share link"
                style={{
                  flex: 1,
                  minWidth: 0,
                  padding: "6px 10px",
                  fontSize: 13,
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                }}
              />
              <button onClick={copyLink} className="detail-btn">
                Copy link
              </button>
            </div>
          )}
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Embed</span>
          <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
            <input
              type="text"
              readOnly
              value={embedSnippet}
              aria-label="Embed code"
              style={{
                flex: 1,
                minWidth: 0,
                padding: "6px 10px",
                fontSize: 13,
                background: "var(--color-bg)",
                border: "1px solid var(--color-border)",
                borderRadius: 4,
                color: "var(--color-text)",
              }}
            />
            <button onClick={copyEmbed} className="detail-btn">
              Copy embed
            </button>
          </div>
        </div>

        {!isViewer && (
          <>
            <div className="detail-setting-row">
              <span className="detail-setting-label">Password</span>
              <div className="detail-setting-value">
                <span>
                  {video.hasPassword ? "Password set" : "No password"}
                </span>
                {video.hasPassword ? (
                  <button onClick={removePassword} className="detail-btn">
                    Remove password
                  </button>
                ) : (
                  <button onClick={addPassword} className="detail-btn">
                    Set password
                  </button>
                )}
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Expiry</span>
              <div className="detail-setting-value">
                <span>{expiry.text}</span>
                <button onClick={toggleLinkExpiry} className="detail-btn">
                  {video.shareExpiresAt === null
                    ? "Set expiry"
                    : "Remove expiry"}
                </button>
                {video.shareExpiresAt !== null && (
                  <button onClick={extendVideo} className="detail-btn">
                    Extend
                  </button>
                )}
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Downloads</span>
              <button
                onClick={toggleDownload}
                className={`detail-toggle${video.downloadEnabled ? " detail-toggle--active" : ""}`}
              >
                {video.downloadEnabled ? "Enabled" : "Disabled"}
              </button>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Email gate</span>
              <button
                onClick={toggleEmailGate}
                className={`detail-toggle${video.emailGateEnabled ? " detail-toggle--active" : ""}`}
              >
                {video.emailGateEnabled ? "Enabled" : "Disabled"}
              </button>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Comments</span>
              <select
                aria-label="Comment mode"
                value={video.commentMode}
                onChange={(e) => changeCommentMode(e.target.value)}
                style={{
                  background: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color:
                    video.commentMode !== "disabled"
                      ? "var(--color-accent)"
                      : "var(--color-text-secondary)",
                  fontSize: 13,
                  padding: "4px 8px",
                  cursor: "pointer",
                }}
              >
                <option value="disabled">Off</option>
                <option value="anonymous">Anonymous</option>
                <option value="name_required">Name required</option>
                <option value="name_email_required">Name + email</option>
              </select>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Thumbnail</span>
              <div className="detail-setting-value">
                <label
                  style={{
                    cursor: uploadingThumbnail ? "default" : "pointer",
                  }}
                >
                  <span className="detail-btn" role="button" tabIndex={0}>
                    {uploadingThumbnail ? "Uploading..." : "Upload"}
                  </span>
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    style={{ display: "none" }}
                    disabled={uploadingThumbnail}
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) uploadThumbnail(file);
                      e.target.value = "";
                    }}
                  />
                </label>
                {video.thumbnailUrl && (
                  <button
                    onClick={resetThumbnail}
                    disabled={uploadingThumbnail}
                    className="detail-btn"
                  >
                    Reset thumbnail
                  </button>
                )}
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Notifications</span>
              <select
                aria-label="View notifications"
                value={video.viewNotification ?? ""}
                onChange={(e) => changeNotification(e.target.value)}
                style={{
                  background: "var(--color-surface)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text-secondary)",
                  fontSize: 13,
                  padding: "4px 8px",
                  cursor: "pointer",
                }}
              >
                <option value="">Account default</option>
                <option value="off">Off</option>
                <option value="every">Every view</option>
                <option value="digest">Daily digest</option>
              </select>
            </div>

            {limits?.brandingEnabled && (
              <div className="detail-setting-row">
                <span className="detail-setting-label">Branding</span>
                <button onClick={openBranding} className="detail-btn">
                  Customize
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Branding Modal */}
      {brandingOpen && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "var(--color-overlay)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 100,
          }}
          onClick={() => setBrandingOpen(false)}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--color-surface)",
              borderRadius: 12,
              padding: 24,
              width: "calc(100vw - 32px)",
              maxWidth: 400,
              maxHeight: "80vh",
              overflow: "auto",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                color: "var(--color-text)",
                fontSize: 18,
                margin: "0 0 16px",
              }}
            >
              Video Branding
            </h3>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 13,
                margin: "0 0 16px",
              }}
            >
              Override your account branding for this video. Leave empty to
              inherit.
            </p>

            <label
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 4,
                marginBottom: 12,
              }}
            >
              <span
                style={{
                  color: "var(--color-text-secondary)",
                  fontSize: 13,
                }}
              >
                Company name
              </span>
              <input
                type="text"
                value={videoBranding.companyName ?? ""}
                onChange={(e) =>
                  setVideoBranding({
                    ...videoBranding,
                    companyName: e.target.value || null,
                  })
                }
                placeholder="Inherit from account"
                maxLength={limits?.fieldLimits?.companyName ?? 200}
                style={{
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                  padding: "8px 12px",
                  fontSize: 14,
                  width: "100%",
                }}
              />
            </label>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 8,
                marginBottom: 12,
              }}
            >
              {(
                [
                  "colorBackground",
                  "colorSurface",
                  "colorText",
                  "colorAccent",
                ] as const
              ).map((key) => {
                const labels: Record<string, string> = {
                  colorBackground: "Background",
                  colorSurface: "Surface",
                  colorText: "Text",
                  colorAccent: "Accent",
                };
                return (
                  <label
                    key={key}
                    style={{
                      display: "flex",
                      flexDirection: "column",
                      gap: 4,
                    }}
                  >
                    <span
                      style={{
                        color: "var(--color-text-secondary)",
                        fontSize: 13,
                      }}
                    >
                      {labels[key]}
                    </span>
                    <input
                      type="text"
                      value={videoBranding[key] ?? ""}
                      onChange={(e) =>
                        setVideoBranding({
                          ...videoBranding,
                          [key]: e.target.value || null,
                        })
                      }
                      placeholder="Inherit"
                      style={{
                        background: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: 4,
                        color: "var(--color-text)",
                        padding: "6px 10px",
                        fontSize: 13,
                        width: "100%",
                      }}
                    />
                  </label>
                );
              })}
            </div>

            <label
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 4,
                marginBottom: 16,
              }}
            >
              <span
                style={{
                  color: "var(--color-text-secondary)",
                  fontSize: 13,
                }}
              >
                Footer text
              </span>
              <input
                type="text"
                value={videoBranding.footerText ?? ""}
                onChange={(e) =>
                  setVideoBranding({
                    ...videoBranding,
                    footerText: e.target.value || null,
                  })
                }
                placeholder="Inherit from account"
                maxLength={limits?.fieldLimits?.footerText ?? 500}
                style={{
                  background: "var(--color-bg)",
                  border: "1px solid var(--color-border)",
                  borderRadius: 4,
                  color: "var(--color-text)",
                  padding: "8px 12px",
                  fontSize: 14,
                  width: "100%",
                }}
              />
            </label>

            {brandingMessage && (
              <p
                style={{
                  color:
                    brandingMessage === "Saved"
                      ? "var(--color-accent)"
                      : "var(--color-error)",
                  fontSize: 13,
                  margin: "0 0 12px",
                }}
              >
                {brandingMessage}
              </p>
            )}

            <div style={{ display: "flex", gap: 8 }}>
              <button
                onClick={saveBranding}
                disabled={savingBranding}
                className="detail-btn detail-btn--accent"
              >
                {savingBranding ? "Saving..." : "Save"}
              </button>
              <button
                onClick={() => setBrandingOpen(false)}
                className="detail-btn"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <Toast message={toast.message} />

      {confirmDialog && (
        <ConfirmDialog
          message={confirmDialog.message}
          confirmLabel={confirmDialog.confirmLabel}
          danger={confirmDialog.danger}
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}

      {promptDialog && (
        <PromptDialog
          title={promptDialog.title}
          placeholder={promptDialog.placeholder}
          submitLabel={promptDialog.submitLabel}
          onSubmit={promptDialog.onSubmit}
          onCancel={() => setPromptDialog(null)}
        />
      )}
    </>
  );
}
