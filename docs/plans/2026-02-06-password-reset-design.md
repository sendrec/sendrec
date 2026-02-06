# Password Reset Flow — Design

## Goal

Allow users to reset their password via email when they've forgotten it.

## User Flow

Two stages:

1. **Request** — User clicks "Forgot password?" on login, enters email, submits. App generates a single-use reset token, stores its hash in the database, sends a reset link via Listmonk. Response is always "If an account exists, we've sent a reset link" (prevents user enumeration).

2. **Reset** — User clicks link in email, opens `/reset-password?token=<token>`. Enters new password with confirmation. App validates token (exists, not expired, not used), updates password hash, marks token used, revokes all refresh tokens (forces re-login everywhere). Redirects to login.

## Database Schema

Migration `000008_create_password_resets`:

```sql
CREATE TABLE password_resets (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
```

- Store SHA-256 hash of token, not plaintext
- `used_at` instead of deleting — audit trail, race condition prevention
- Index on `user_id` for invalidating old tokens

## Backend API

### `POST /api/auth/forgot-password`

- Input: `{"email": "user@example.com"}`
- Looks up user by email. If not found, return 200 anyway.
- Invalidates existing unused reset tokens for that user
- Generates 32-byte random token, stores SHA-256 hash with 1-hour expiry
- Calls Listmonk transactional API to send email with link: `{baseURL}/reset-password?token={rawToken}`
- Returns `200 {"message": "If an account exists, we've sent a reset link"}`
- Rate limited: 0.5 req/sec, burst 5

### `POST /api/auth/reset-password`

- Input: `{"token": "...", "password": "newpass"}`
- Hashes token with SHA-256, looks up in `password_resets` where `used_at IS NULL AND expires_at > NOW()`
- Not found/expired: `400 "invalid or expired reset link"`
- Validates password (8-72 chars)
- Updates user's password hash (bcrypt)
- Marks token used (`used_at = NOW()`)
- Revokes all refresh tokens for that user
- Returns `200 {"message": "Password updated successfully"}`
- Rate limited: 0.5 req/sec, burst 5

## Listmonk Integration

New `internal/email/` package:

- `Client` struct: `baseURL`, `username`, `password`
- Method: `SendPasswordReset(ctx, toEmail, toName, resetLink) error`
- Calls `POST /api/tx` on Listmonk with template ID
- Config: `LISTMONK_URL`, `LISTMONK_USER`, `LISTMONK_PASSWORD`, `LISTMONK_TEMPLATE_ID`
- If `LISTMONK_URL` not set: logs reset link to stdout (dev/self-hosting fallback)
- If send fails: log error, still return 200 (no enumeration)

## Frontend

### `ForgotPassword.tsx` (`/forgot-password`)

- Email-only form
- On success: shows confirmation message ("Check your email for a reset link")
- Link back to login

### `ResetPassword.tsx` (`/reset-password`)

- Reads `token` from URL query params
- No token: error with link to forgot-password
- Form: new password + confirm password
- Client-side validation: match, min 8 chars
- On success: "Password updated" with link to login
- On error: message with link to request new reset

### Login.tsx changes

- Add "Forgot password?" link below password field

### Routing

- Two new public routes: `/forgot-password`, `/reset-password`

## Email Template

Transactional template in Listmonk admin UI:

- Subject: "Reset your SendRec password"
- Body: brief message, green CTA button with reset URL
- Footer: "If you didn't request this, ignore this email. Link expires in 1 hour."
- Reset link passed via `{{ .Tx.Data.resetLink }}`
- Setup: create template in Listmonk, set `LISTMONK_TEMPLATE_ID` in `.env`

## Security

- No user enumeration (always 200 on forgot-password)
- Token stored as SHA-256 hash (raw only in email)
- Single-use tokens (marked used on reset)
- 1-hour expiry
- Old tokens invalidated on new request
- All refresh tokens revoked on password change
- Rate limiting on both endpoints
- Listmonk called over Docker internal network

## Intentionally Omitted (YAGNI)

- Email verification on registration
- "Password changed" confirmation email
- Account lockout after failed resets
- Admin UI for reset tokens
