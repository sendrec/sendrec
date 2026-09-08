import { apiFetch } from "../../api/client";
import { copyToClipboard } from "../../utils/clipboard";
import { ConfirmDialogState } from "../../components/ConfirmDialog";

interface PlaylistSharingData {
  isShared: boolean;
  shareToken?: string;
  shareUrl?: string;
  hasPassword: boolean;
  requireEmail: boolean;
}

interface PlaylistSharingProps {
  playlistId: string;
  playlist: PlaylistSharingData;
  onPlaylistUpdate: (updates: Partial<PlaylistSharingData>) => void;
  onPlaylistRefresh: () => Promise<void>;
  showToast: (message: string) => void;
  setConfirmDialog: (dialog: ConfirmDialogState | null) => void;
  setPromptDialog: (dialog: {
    title: string;
    onSubmit: (value: string) => void;
    placeholder?: string;
    submitLabel?: string;
  } | null) => void;
}

export function PlaylistSharing({
  playlistId,
  playlist,
  onPlaylistUpdate,
  onPlaylistRefresh,
  showToast,
  setConfirmDialog,
  setPromptDialog,
}: PlaylistSharingProps) {
  async function toggleSharing() {
    const newValue = !playlist.isShared;
    await apiFetch(`/api/playlists/${playlistId}`, {
      method: "PATCH",
      body: JSON.stringify({ isShared: newValue }),
    });
    await onPlaylistRefresh();
    showToast(newValue ? "Freigabe aktiviert" : "Freigabe deaktiviert");
  }

  async function toggleEmailGate() {
    const newValue = !playlist.requireEmail;
    await apiFetch(`/api/playlists/${playlistId}`, {
      method: "PATCH",
      body: JSON.stringify({ requireEmail: newValue }),
    });
    onPlaylistUpdate({ requireEmail: newValue });
  }

  function setSharePassword() {
    setPromptDialog({
      title: "Passwort für diese Playlist eingeben:",
      placeholder: "Passwort",
      submitLabel: "Passwort setzen",
      onSubmit: async (password) => {
        setPromptDialog(null);
        await apiFetch(`/api/playlists/${playlistId}`, {
          method: "PATCH",
          body: JSON.stringify({ sharePassword: password }),
        });
        onPlaylistUpdate({ hasPassword: true });
        showToast("Passwort gesetzt");
      },
    });
  }

  function removeSharePassword() {
    setConfirmDialog({
      message: "Passwort von dieser Playlist entfernen?",
      confirmLabel: "Entfernen",
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null);
        await apiFetch(`/api/playlists/${playlistId}`, {
          method: "PATCH",
          body: JSON.stringify({ sharePassword: "" }),
        });
        onPlaylistUpdate({ hasPassword: false });
        showToast("Passwort entfernt");
      },
    });
  }

  return (
    <div className="video-detail-section">
      <h2 className="video-detail-section-title">Freigabe</h2>

      <div className="detail-setting-row">
        <span className="detail-setting-label">Öffentlicher Link</span>
        <button
          onClick={toggleSharing}
          className={`detail-toggle${playlist.isShared ? " detail-toggle--active" : ""}`}
        >
          {playlist.isShared ? "Enabled" : "Deaktiviert"}
        </button>
      </div>

      {playlist.isShared && playlist.shareUrl && (
        <>
          <div className="detail-setting-row">
            <span className="detail-setting-label">Freigabelink</span>
            <div
              style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}
            >
              <input
                type="text"
                readOnly
                value={playlist.shareUrl}
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
              <button
                onClick={() => {
                  copyToClipboard(playlist.shareUrl!);
                  showToast("Link kopiert");
                }}
                className="detail-btn"
              >
                Link kopieren
              </button>
            </div>
          </div>

          <div className="detail-setting-row">
            <span className="detail-setting-label">Einbetten</span>
            <div
              style={{ display: "flex", gap: 8, flex: 1, minWidth: 0 }}
            >
              <input
                type="text"
                readOnly
                value={`<iframe src="${window.location.origin}/embed/playlist/${playlist.shareToken}" width="800" height="450" frameborder="0" allowfullscreen></iframe>`}
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
              <button
                onClick={() => {
                  copyToClipboard(
                    `<iframe src="${window.location.origin}/embed/playlist/${playlist.shareToken}" width="800" height="450" frameborder="0" allowfullscreen></iframe>`,
                  );
                  showToast("Einbettungscode kopiert");
                }}
                className="detail-btn"
              >
                Einbettung kopieren
              </button>
            </div>
          </div>

          <div className="detail-setting-row">
            <span className="detail-setting-label">Passwort</span>
            <div className="detail-setting-value">
              <span>
                {playlist.hasPassword ? "Passwort gesetzt" : "Kein Passwort"}
              </span>
              {playlist.hasPassword ? (
                <button
                  onClick={removeSharePassword}
                  className="detail-btn"
                >
                  Passwort entfernen
                </button>
              ) : (
                <button
                  onClick={setSharePassword}
                  className="detail-btn"
                >
                  Passwort setzen
                </button>
              )}
            </div>
          </div>

          <div className="detail-setting-row">
            <span className="detail-setting-label">E-Mail-Abfrage</span>
            <button
              onClick={toggleEmailGate}
              className={`detail-toggle${playlist.requireEmail ? " detail-toggle--active" : ""}`}
            >
              {playlist.requireEmail ? "Enabled" : "Deaktiviert"}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
