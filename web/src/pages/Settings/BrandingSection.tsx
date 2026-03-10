import { type FormEvent, useState } from "react";
import { apiFetch } from "../../api/client";
import { LimitsResponse } from "../../types/limits";
import { BrandingSettings } from "./types";

const hexColorPattern = /^#[0-9a-fA-F]{6}$/;

interface BrandingSectionProps {
  initialBranding: BrandingSettings;
  limits: LimitsResponse | null;
}

export function BrandingSection({ initialBranding, limits }: BrandingSectionProps) {
  const [branding, setBranding] = useState(initialBranding);
  const [brandingMessage, setBrandingMessage] = useState("");
  const [brandingError, setBrandingError] = useState("");
  const [savingBranding, setSavingBranding] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);

  async function handleBrandingSave(event: FormEvent) {
    event.preventDefault();
    setBrandingError("");
    setBrandingMessage("");

    for (const [key, value] of Object.entries(branding)) {
      if (key.startsWith("color") && value && !hexColorPattern.test(value)) {
        setBrandingError(`Invalid color for ${key}`);
        return;
      }
    }

    setSavingBranding(true);
    try {
      await apiFetch("/api/settings/branding", {
        method: "PUT",
        body: JSON.stringify({
          companyName: branding.companyName || null,
          logoKey: branding.logoKey === "none" ? "none" : branding.logoKey || null,
          colorBackground: branding.colorBackground || null,
          colorSurface: branding.colorSurface || null,
          colorText: branding.colorText || null,
          colorAccent: branding.colorAccent || null,
          footerText: branding.footerText || null,
          customCss: branding.customCss || null,
        }),
      });
      setBrandingMessage("Branding saved");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to save branding");
    } finally {
      setSavingBranding(false);
    }
  }

  function handleBrandingReset() {
    setBranding({
      companyName: null, logoKey: null,
      colorBackground: null, colorSurface: null, colorText: null, colorAccent: null,
      footerText: null, customCss: null,
    });
  }

  async function handleLogoUpload(file: File) {
    if (file.type !== "image/png" && file.type !== "image/svg+xml") {
      setBrandingError("Logo must be PNG or SVG");
      return;
    }
    if (file.size > 512 * 1024) {
      setBrandingError("Logo must be 512KB or smaller");
      return;
    }

    setUploadingLogo(true);
    setBrandingError("");
    try {
      const result = await apiFetch<{ uploadUrl: string; logoKey: string }>("/api/settings/branding/logo", {
        method: "POST",
        body: JSON.stringify({ contentType: file.type, contentLength: file.size }),
      });
      if (!result) throw new Error("Failed to get upload URL");

      const uploadResp = await fetch(result.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!uploadResp.ok) throw new Error("Failed to upload logo");

      setBranding((prev) => ({ ...prev, logoKey: result.logoKey }));
      setBrandingMessage("Logo uploaded");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to upload logo");
    } finally {
      setUploadingLogo(false);
    }
  }

  async function handleLogoRemove() {
    setBrandingError("");
    try {
      await apiFetch("/api/settings/branding/logo", { method: "DELETE" });
      setBranding((prev) => ({ ...prev, logoKey: null }));
      setBrandingMessage("Logo removed");
    } catch (err) {
      setBrandingError(err instanceof Error ? err.message : "Failed to remove logo");
    }
  }

  return (
    <form
      onSubmit={handleBrandingSave}
      className="card settings-section"
    >
      <h2>Branding</h2>
      <p className="card-description">
        Customize how your shared video pages look to viewers.
      </p>

      <div className="form-field">
        <label className="form-label">Company name</label>
        <input
          type="text"
          className="form-input"
          value={branding.companyName ?? ""}
          onChange={(e) => setBranding({ ...branding, companyName: e.target.value || null })}
          placeholder="SendRec"
          maxLength={limits?.fieldLimits?.companyName ?? 200}
        />
      </div>

      <div className="logo-section">
        <span className="logo-section-label">Logo</span>
        <div className="logo-section-controls">
          {branding.logoKey && branding.logoKey !== "none" ? (
            <>
              <span className="logo-section-name">
                {branding.logoKey.split("/").pop()}
              </span>
              <button
                type="button"
                className="btn btn--danger btn--danger-sm"
                onClick={handleLogoRemove}
              >
                Remove
              </button>
            </>
          ) : branding.logoKey === "none" ? (
            <>
              <span className="logo-section-status">Logo hidden</span>
              <button
                type="button"
                className="btn btn--secondary"
                onClick={handleLogoRemove}
              >
                Show default logo
              </button>
            </>
          ) : (
            <>
              <label className="btn btn--secondary" style={{ cursor: uploadingLogo ? "default" : "pointer" }}>
                {uploadingLogo ? "Uploading..." : "Upload logo (PNG or SVG, max 512KB)"}
                <input
                  type="file"
                  accept="image/png,image/svg+xml"
                  className="sr-only"
                  disabled={uploadingLogo}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) handleLogoUpload(file);
                    e.target.value = "";
                  }}
                />
              </label>
              <button
                type="button"
                className="btn btn--secondary"
                onClick={() => setBranding((prev) => ({ ...prev, logoKey: "none" }))}
              >
                Hide logo
              </button>
            </>
          )}
        </div>
      </div>

      <div className="form-field">
        <label className="form-label">Footer text</label>
        <textarea
          className="form-input"
          value={branding.footerText ?? ""}
          onChange={(e) => setBranding({ ...branding, footerText: e.target.value || null })}
          placeholder="Custom footer message"
          maxLength={limits?.fieldLimits?.footerText ?? 500}
          rows={2}
        />
      </div>

      <div className="color-grid">
        {(["colorBackground", "colorSurface", "colorText", "colorAccent"] as const).map((key) => {
          const labels: Record<string, string> = {
            colorBackground: "Background",
            colorSurface: "Surface",
            colorText: "Text",
            colorAccent: "Accent",
          };
          const defaults: Record<string, string> = {
            colorBackground: "#0a1628",
            colorSurface: "#1e293b",
            colorText: "#ffffff",
            colorAccent: "#00b67a",
          };
          return (
            <div key={key} className="color-field">
              <span className="form-label">{labels[key]}</span>
              <div className="color-row">
                <input
                  type="color"
                  className="color-swatch"
                  value={branding[key] ?? defaults[key]}
                  onChange={(e) => setBranding({ ...branding, [key]: e.target.value })}
                />
                <input
                  type="text"
                  className="form-input"
                  value={branding[key] ?? ""}
                  onChange={(e) => setBranding({ ...branding, [key]: e.target.value || null })}
                  placeholder={defaults[key]}
                  style={{ flex: 1 }}
                />
              </div>
            </div>
          );
        })}
      </div>

      <div
        className="branding-preview"
        style={{ background: branding.colorBackground ?? "#0a1628" }}
      >
        <p className="branding-preview-label">Preview</p>
        <div className="branding-preview-title" style={{ color: branding.colorAccent ?? "#00b67a" }}>
          {branding.companyName || "SendRec"}
        </div>
        <div className="branding-preview-card" style={{ background: branding.colorSurface ?? "#1e293b" }}>
          <span style={{ color: branding.colorText ?? "#ffffff", fontSize: 14 }}>Sample video title</span>
        </div>
      </div>

      <div className="form-field">
        <label className="form-label">Custom CSS</label>
        <textarea
          className="form-input form-input--mono"
          value={branding.customCss ?? ""}
          onChange={(e) => setBranding({ ...branding, customCss: e.target.value || null })}
          placeholder={"/* Override watch page styles */\nbody { font-family: 'Inter', sans-serif; }\n.download-btn { border-radius: 20px; }\n.comment-submit { border-radius: 20px; }"}
          maxLength={limits?.fieldLimits?.customCSS ?? 10240}
          rows={6}
        />
        <span className="form-hint">
          Injected into the watch page &lt;style&gt; tag. Max 10KB. No @import url() or closing style tags.
        </span>
        <details className="settings-details">
          <summary>Available CSS selectors</summary>
          <pre>{`/* CSS Variables */
:root {
  --brand-bg;       /* Page background */
  --brand-surface;  /* Cards, panels */
  --brand-text;     /* Primary text */
  --brand-accent;   /* Buttons, links */
  --player-accent;  /* Seek bar, progress */
}

/* Layout */
body                /* Background, font, text color */
.container          /* Max-width wrapper (960px) */
.video-title        /* Video heading */
.video-meta         /* Creator info row */
.video-meta-avatar  /* Creator avatar */
.video-meta-name    /* Creator name */

/* Header & Footer */
.logo               /* Logo + name link */
.logo img           /* Logo image */
.branding           /* "Shared via SendRec" footer */
.branding a         /* Footer link */

/* Video Player */
.player-container   /* Player wrapper */
.player-overlay     /* Play button overlay */
.play-overlay-btn   /* Large play button */
.player-controls    /* Control bar */
.ctrl-btn           /* Control buttons */
.time-display       /* Current / duration */
.seek-bar           /* Seek bar wrapper */
.seek-track         /* Track background */
.seek-progress      /* Play progress */
.seek-buffered      /* Buffered range */
.seek-thumb         /* Draggable handle */
.volume-group       /* Volume control */
.volume-slider      /* Volume slider */
.speed-dropdown     /* Speed selector */
.speed-menu         /* Speed options dropdown */
.speed-menu button.active /* Selected speed */

/* Seek Bar Overlays */
.seek-chapters      /* Chapter markers */
.seek-chapter       /* Single chapter */
.seek-markers       /* Comment markers */
.seek-marker        /* Single marker dot */

/* Actions */
.actions            /* Download + controls */
.download-btn       /* Download button */

/* Comments */
.comments-section   /* Full comments area */
.comments-header    /* Heading */
.comment            /* Single comment */
.comment-meta       /* Author + badges */
.comment-author     /* Commenter name */
.comment-body       /* Comment text */
.comment-owner-badge   /* "Owner" badge */
.comment-private-badge /* "Private" badge */
.comment-timestamp  /* Timestamp link */
.comment-form       /* New comment form */
.comment-form input /* Name + email fields */
.comment-form textarea /* Text area */
.comment-submit     /* "Post comment" button */
.no-comments        /* Empty state text */

/* Reactions */
.reaction-bar       /* Quick-reaction buttons */
.reaction-btn       /* Single reaction */

/* Emoji Picker */
.emoji-trigger      /* Emoji button */
.emoji-grid         /* Dropdown panel */
.emoji-btn          /* Single emoji */

/* Transcript & Summary */
.transcript-section /* Full panel area */
.panel-tabs         /* Summary/Transcript tabs */
.panel-tab          /* Tab button */
.panel-tab--active  /* Active tab */
.summary-text       /* Summary paragraph */
.chapter-list       /* Chapters container */
.chapter-item       /* Single chapter */
.chapter-item.active /* Playing chapter */
.chapter-timestamp  /* Chapter time */
.chapter-title      /* Chapter name */
.transcript-header  /* Heading */
.transcript-segment /* Single line */
.transcript-segment.active /* Playing line */
.transcript-timestamp /* Time link */
.transcript-text    /* Segment text */

/* Call to Action */
.cta-card           /* CTA container */
.cta-title          /* CTA heading */
.cta-desc           /* CTA description */
.cta-btn            /* CTA button */

/* Utilities */
.hidden             /* display: none */

/* Responsive & Accessibility */
@media (max-width: 640px) { ... }
@media (prefers-reduced-motion: reduce) { ... }`}</pre>
        </details>
      </div>

      {brandingError && (
        <p className="status-message status-message--error">{brandingError}</p>
      )}
      {brandingMessage && (
        <p className="status-message status-message--success">{brandingMessage}</p>
      )}

      <div className="btn-row">
        <button
          type="submit"
          className="btn btn--primary"
          disabled={savingBranding}
        >
          {savingBranding ? "Saving..." : "Save branding"}
        </button>
        <button
          type="button"
          className="btn btn--secondary"
          onClick={handleBrandingReset}
        >
          Reset to defaults
        </button>
      </div>
    </form>
  );
}
