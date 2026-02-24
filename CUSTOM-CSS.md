# Custom CSS Guide

SendRec lets you inject custom CSS into your video watch pages to match your brand. This guide covers how to set it up, what you can customize, and includes ready-to-use examples.

## Prerequisites

Custom CSS is part of the branding feature. Your instance must have branding enabled:

```env
BRANDING_ENABLED=true
```

When disabled, the branding section (including Custom CSS) does not appear in Settings or the Library.

## How it works

1. Go to **Settings > Watch Page Branding**
2. Enter your CSS in the **Custom CSS** textarea
3. Click **Save branding**

Your CSS is appended to the end of the watch page `<style>` tag, so it overrides the default styles. It applies to all your videos as a user-level default. There is no per-video CSS override — use the per-video branding overrides (company name, colors, logo, footer) for video-specific adjustments.

## Limits

| Rule | Limit |
|------|-------|
| Maximum size | 10 KB |
| `</style>` tags | Not allowed (prevents HTML injection) |
| `@import url()` | Not allowed (prevents external resource loading) |

## CSS variables

The watch page uses four CSS variables that are set from your branding color settings. You can override them in custom CSS, but it's easier to use the color pickers in Settings.

```css
:root {
  --brand-bg: #0f172a;      /* Page background */
  --brand-surface: #1e293b;  /* Cards, inputs, section borders */
  --brand-text: #f8fafc;     /* Primary text color */
  --brand-accent: #00b67a;   /* Buttons, links, active states */
}
```

## Watch page structure

```
body
  .container
    a.logo                    ← Company logo + name
      img                     ← Logo image (20x20)
    .player-container         ← Video player wrapper
      video#player            ← Video element (native controls on iOS)
      .player-overlay         ← Large center play button overlay
      .player-spinner         ← Loading spinner
      .player-error           ← Error state overlay
      .player-controls        ← Custom controls bar (hidden on iOS)
        .ctrl-btn#play-btn    ← Play/pause
        .time-display         ← Current time / duration
        .seek-bar             ← Seek bar with chapters, markers, tooltip
        .volume-group         ← Volume button + slider
        .speed-dropdown       ← Speed selector (0.5x–2x)
        .ctrl-btn#pip-btn     ← Picture-in-picture
        .shortcuts-wrapper    ← Keyboard shortcuts panel
        .ctrl-btn#fullscreen-btn ← Fullscreen toggle
    h1                        ← Video title
    p.meta                    ← "Creator name · Feb 17, 2026"
    .actions                  ← Button row
      .download-btn           ← Download button
    .cta-card                 ← CTA card (shown on video end)
      .cta-dismiss            ← CTA dismiss button
      .cta-btn                ← CTA action button
    .comments-section         ← Comments area
      h2.comments-header      ← "Comments" heading
      .comment                ← Individual comment card
        .comment-meta         ← Author + badges
          .comment-author     ← Commenter name
          .comment-owner-badge← "Owner" pill
          .comment-private-badge ← "Private" pill
          .comment-timestamp  ← Clickable timestamp pill
        .comment-body         ← Comment text
      .comment-form           ← New comment form
        .form-row             ← Name + email row
          input               ← Name / email fields
        .timestamp-toggle     ← "Add timestamp" toggle
        textarea              ← Comment text area
        .comment-form-actions ← Submit row
          .emoji-picker-wrapper
            .emoji-trigger    ← Emoji button
            .emoji-grid       ← Emoji dropdown
              .emoji-btn      ← Individual emoji
          .comment-submit     ← "Post comment" button
    .transcript-section       ← Transcript area
      h2.transcript-header    ← "Transcript" heading
      .transcript-segment     ← Single transcript line
        .transcript-timestamp ← Timestamp (e.g. "1:23")
        .transcript-text      ← Transcript text
    p.branding                ← Footer: "Shared via SendRec"
      a                       ← Footer link
```

> **Note:** On iOS Safari, custom controls are hidden and the native `<video controls>` are used instead for reliable playback.

## Selector reference

### Layout

| Selector | Description | Default |
|----------|-------------|---------|
| `body` | Page background, font, text color | System font stack, `var(--brand-bg)` |
| `.container` | Content wrapper | `max-width: 960px`, `padding: 2rem 1rem` |
| `video` | Video player | `border-radius: 8px` |
| `h1` | Video title | `font-size: 1.5rem`, `font-weight: 600` |
| `.meta` | Creator + date line | `color: #94a3b8`, `font-size: 0.875rem` |

### Header and footer

| Selector | Description | Default |
|----------|-------------|---------|
| `.logo` | Company logo link | `color: #94a3b8`, `font-size: 0.8rem` |
| `.logo img` | Logo image | `20px` x `20px` |
| `.branding` | Footer text | `color: #64748b`, `font-size: 0.75rem` |
| `.branding a` | Footer link | `color: var(--brand-accent)` |

### Custom player controls

The custom player controls bar is shown on desktop browsers. On iOS Safari, native `<video controls>` are used instead and these selectors have no effect.

| Selector | Description | Default |
|----------|-------------|---------|
| `.player-controls` | Controls bar container | Gradient background, auto-hide after 3s |
| `.player-controls.hidden` | Hidden state | `opacity: 0`, `pointer-events: none` |
| `.ctrl-btn` | Control button (play, mute, PiP, fullscreen) | White, `18px` |
| `.seek-bar` | Seek bar container | Flex, `height: 20px` |
| `.seek-progress` | Seek progress fill | `var(--player-accent, #00b67a)` |
| `.seek-thumb` | Seek thumb (shown on hover) | `14px` circle, accent color |
| `.seek-chapter` | Chapter segment in seek bar | `rgba(255, 255, 255, 0.15)` |
| `.seek-marker` | Comment marker in seek bar | `6px` wide, accent color |
| `.speed-menu` | Speed dropdown menu | Dark background, `6px` radius |
| `.volume-slider` | Volume slider | `60px` wide |
| `.player-overlay` | Large center play button | Full-size overlay |
| `.player-spinner` | Loading spinner | Animated border circle |
| `.player-error` | Error state message | Centered text with icon |

### Action buttons

| Selector | Description | Default |
|----------|-------------|---------|
| `.actions` | Button row container | `display: flex`, `gap: 1rem` |
| `.download-btn` | Download button | Outlined, `var(--brand-accent)` border |

### Call to action

The CTA card appears when the video finishes playing. Set it per video via Library > overflow menu > "Call to action".

| Selector | Description | Default |
|----------|-------------|---------|
| `.cta-card` | CTA container (hidden until video ends) | `background: var(--brand-surface)`, `border: 1px solid var(--brand-accent)`, `border-radius: 8px` |
| `.cta-card.visible` | CTA container when shown | `display: block` |
| `.cta-btn` | CTA action button | `background: var(--brand-accent)`, `color: #fff`, `border-radius: 6px`, `font-weight: 600` |
| `.cta-dismiss` | CTA dismiss button (top-right "x") | `color: #94a3b8`, `font-size: 1.25rem` |

### Comments

| Selector | Description | Default |
|----------|-------------|---------|
| `.comments-section` | Full comments area | `border-top: 1px solid var(--brand-surface)` |
| `.comments-header` | "Comments" heading | `font-size: 1.125rem` |
| `.comment` | Comment card | `background: var(--brand-surface)`, `border-radius: 8px` |
| `.comment-meta` | Author + badges row | `font-size: 0.8125rem`, `color: #94a3b8` |
| `.comment-author` | Commenter name | `font-weight: 600`, `color: #e2e8f0` |
| `.comment-body` | Comment text | `color: #cbd5e1`, `font-size: 0.9375rem` |
| `.comment-owner-badge` | "Owner" pill | `background: var(--brand-accent)` |
| `.comment-private-badge` | "Private" pill | `background: #3b82f6` |
| `.comment-timestamp` | Timestamp pill | `background: var(--brand-accent)`, clickable |
| `.comment-form` | New comment form | |
| `.comment-form input` | Name / email fields | `background: var(--brand-surface)` |
| `.comment-form textarea` | Comment text area | `min-height: 80px` |
| `.comment-submit` | "Post comment" button | `background: var(--brand-accent)` |
| `.no-comments` | Empty state text | `color: #64748b` |

### Comment markers

Comment markers are displayed inside the seek bar (not as a separate bar).

| Selector | Description | Default |
|----------|-------------|---------|
| `.seek-marker` | Comment position marker in seek bar | `background: var(--player-accent)`, `6px` wide |
| `.seek-marker-tooltip` | Hover tooltip on marker | Dark tooltip with border |

### Emoji picker

| Selector | Description | Default |
|----------|-------------|---------|
| `.emoji-trigger` | Emoji button | `border: 1px solid #334155` |
| `.emoji-grid` | Emoji dropdown panel | `width: 260px`, dark background |
| `.emoji-category` | Category header in picker | Uppercase, `#475569` |
| `.emoji-btn` | Individual emoji | `2rem` x `2rem` |

### Transcript

| Selector | Description | Default |
|----------|-------------|---------|
| `.transcript-section` | Full transcript area | `border-top: 1px solid var(--brand-surface)` |
| `.transcript-header` | "Transcript" heading | `font-size: 1.125rem`, `color: #f8fafc` |
| `.transcript-segment` | Single transcript line | Clickable, `border-radius: 6px` |
| `.transcript-segment.active` | Currently playing line | `background: rgba(0, 182, 122, 0.1)` |
| `.transcript-timestamp` | Timestamp in transcript | `color: var(--brand-accent)` |
| `.transcript-text` | Transcript text | `color: #cbd5e1` |

### Other

| Selector | Description | Default |
|----------|-------------|---------|
| `.timestamp-toggle` | "Add timestamp" toggle | Pill shape, `color: #94a3b8` |
| `.timestamp-toggle.active` | Active timestamp toggle | `color: var(--brand-accent)` |
| `.browser-warning` | Safari WebM warning (desktop: "use Chrome/Firefox"; iOS: "still being processed") | Yellow border, `#fbbf24` text |
| `.hidden` | Hidden elements | `display: none` |

### Mobile breakpoint

The watch page has a responsive breakpoint at `640px`. You can override mobile styles:

```css
@media (max-width: 640px) {
  .container { padding: 1rem 0.5rem; }
  h1 { font-size: 1.1rem; }
}
```

## Examples

### Minimal: change the font

```css
body {
  font-family: 'Georgia', serif;
}
```

### Pill-shaped buttons

```css
.download-btn,
.comment-submit {
  border-radius: 20px;
}
```

### Warm theme

```css
:root {
  --brand-bg: #1a1412;
  --brand-surface: #2a2220;
  --brand-text: #f5e6d3;
  --brand-accent: #e07c3e;
}

body {
  font-family: 'Georgia', serif;
}

video {
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(224, 124, 62, 0.15);
}

.download-btn {
  border-radius: 20px;
}

.comment-submit {
  border-radius: 20px;
}
```

### Light theme

```css
:root {
  --brand-bg: #ffffff;
  --brand-surface: #f1f5f9;
  --brand-text: #1e293b;
  --brand-accent: #2563eb;
}

.meta { color: #64748b; }
.comment-author { color: #1e293b; }
.comment-body { color: #334155; }
.comment-meta { color: #64748b; }
.transcript-text { color: #334155; }
.transcript-header { color: #1e293b; }
.comment-form input,
.comment-form textarea { border-color: #cbd5e1; color: #1e293b; }
.emoji-trigger { border-color: #cbd5e1; }
.emoji-grid { background: #ffffff; border-color: #e2e8f0; }
.marker-tooltip { background: #ffffff; border-color: #e2e8f0; color: #1e293b; }
.logo { color: #64748b; }
.branding { color: #94a3b8; }
.no-comments { color: #94a3b8; }
```

### Corporate: narrow container + subtle video shadow

```css
.container {
  max-width: 720px;
}

video {
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.3);
}

h1 {
  font-size: 1.25rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
```

### Style the CTA card

```css
.cta-card {
  background: linear-gradient(135deg, var(--brand-surface), #1a1a2e);
  border: 2px solid var(--brand-accent);
  border-radius: 16px;
  padding: 2rem;
}

.cta-btn {
  border-radius: 24px;
  padding: 1rem 3rem;
  font-size: 1.125rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
```

### Hide sections

```css
/* Hide comments */
.comments-section { display: none; }

/* Hide transcript */
.transcript-section { display: none; }

/* Hide footer */
.branding { display: none; }

/* Hide CTA card */
.cta-card { display: none !important; }
```

## API

Custom CSS can also be set via the API:

```bash
curl -X PUT https://your-instance.com/api/branding \
  -H "Authorization: Bearer sr_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "customCss": "body { font-family: Georgia, serif; }"
  }'
```

The `customCss` field is included in `GET /api/branding` and `PUT /api/branding`. Set to `null` to clear.
