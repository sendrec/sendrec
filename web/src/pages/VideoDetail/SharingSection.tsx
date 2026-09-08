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
    toast.show("Link kopiert");
  }

  async function copyEmbed() {
    await copyToClipboard(embedSnippet);
    toast.show("Einbettungscode kopiert");
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
    toast.show("Link verlängert");
  }

  function addPassword() {
    setPromptDialog({
      title: "Passwort für dieses Video eingeben:",
      placeholder: "Passwort",
      submitLabel: "Passwort setzen",
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
      message: "Passwort von diesem Video entfernen?",
      confirmLabel: "Entfernen",
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
      toast.show("Vorschaubild aktualisiert");
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
      toast.show("Vorschaubild zurückgesetzt");
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
      setBrandingMessage("Gespeichert");
      setTimeout(() => setBrandingOpen(false), 1000);
    } catch (err) {
      setBrandingMessage(
        err instanceof Error ? err.message : "Speichern fehlgeschlagen",
      );
    } finally {
      setSavingBranding(false);
    }
  }

  return (
    <>
      <div className="video-detail-section">
        <h2 className="video-detail-section-title">Freigabe-Einstellungen</h2>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Freigabelink</span>
          {video.status === "processing" ? (
            <span
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 13,
              }}
            >
              Verfügbar, sobald die Verarbeitung abgeschlossen ist
            </span>
          ) : (
            <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
              <input
                type="text"
                readOnly
                value={video.shareUrl}
                aria-label="Freigabelink"
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
                Link kopieren
              </button>
            </div>
          )}
        </div>

        <div className="detail-setting-row">
          <span className="detail-setting-label">Einbetten</span>
          <div style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}>
            <input
              type="text"
              readOnly
              value={embedSnippet}
              aria-label="Einbettungscode"
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
              Einbettung kopieren
            </button>
          </div>
        </div>

        {!isViewer && (
          <>
            <div className="detail-setting-row">
              <span className="detail-setting-label">Passwort</span>
              <div className="detail-setting-value">
                <span>
                  {video.hasPassword ? "Passwort gesetzt" : "Kein Passwort"}
                </span>
                {video.hasPassword ? (
                  <button onClick={removePassword} className="detail-btn">
                    Passwort entfernen
                  </button>
                ) : (
                  <button onClick={addPassword} className="detail-btn">
                    Passwort setzen
                  </button>
                )}
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Ablauf</span>
              <div className="detail-setting-value">
                <span>{expiry.text}</span>
                <button onClick={toggleLinkExpiry} className="detail-btn">
                  {video.shareExpiresAt === null
                    ? "Ablauf festlegen"
                    : "Ablauf entfernen"}
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
                {video.downloadEnabled ? "Enabled" : "Deaktiviert"}
              </button>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">E-Mail-Abfrage</span>
              <button
                onClick={toggleEmailGate}
                className={`detail-toggle${video.emailGateEnabled ? " detail-toggle--active" : ""}`}
              >
                {video.emailGateEnabled ? "Enabled" : "Deaktiviert"}
              </button>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Kommentare</span>
              <select
                aria-label="Kommentarmodus"
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
                <option value="disabled">Aus</option>
                <option value="anonymous">Anonym</option>
                <option value="name_required">Name erforderlich</option>
                <option value="name_email_required">Name + E-Mail</option>
              </select>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Vorschaubild</span>
              <div className="detail-setting-value">
                <label
                  style={{
                    cursor: uploadingThumbnail ? "default" : "pointer",
                  }}
                >
                  <span className="detail-btn" role="button" tabIndex={0}>
                    {uploadingThumbnail ? "Wird hochgeladen..." : "Upload"}
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
                    Vorschaubild zurücksetzen
                  </button>
                )}
              </div>
            </div>

            <div className="detail-setting-row">
              <span className="detail-setting-label">Benachrichtigungen</span>
              <select
                aria-label="Aufruf-Benachrichtigungen"
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
                <option value="">Kontostandard</option>
                <option value="off">Aus</option>
                <option value="every">Jeder Aufruf</option>
                <option value="digest">Tägliche Zusammenfassung</option>
              </select>
            </div>

            {limits?.brandingEnabled && (
              <div className="detail-setting-row">
                <span className="detail-setting-label">Branding</span>
                <button onClick={openBranding} className="detail-btn">
                  Anpassen
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
              Video-Branding
            </h3>
            <p
              style={{
                color: "var(--color-text-secondary)",
                fontSize: 13,
                margin: "0 0 16px",
              }}
            >
              Überschreibe das Konto-Branding für dieses Video. Leer lassen, um
              die Konto-Einstellungen zu übernehmen.
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
                Firmenname
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
                placeholder="Vom Konto übernehmen"
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
                      placeholder="Vom Konto übernehmen"
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
                Footer-Text
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
                placeholder="Vom Konto übernehmen"
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
                    brandingMessage === "Gespeichert"
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
                {savingBranding ? "Wird gespeichert..." : "Speichern"}
              </button>
              <button
                onClick={() => setBrandingOpen(false)}
                className="detail-btn"
              >
                Abbrechen
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
