# Self-Hosting Guide

SendRec is designed to run on a single server with Docker Compose. A small VPS (2 vCPU, 4 GB RAM) is enough.

## Quick start

```bash
git clone https://github.com/sendrec/sendrec.git
cd sendrec
cp .env.example .env
docker compose -f docker-compose.dev.yml up --build
```

Open http://localhost:8080, register an account, and start recording or uploading videos.

## Standalone binary

SendRec is a single ~32 MB executable with the React frontend, database migrations, and HTML templates all embedded. No Docker required — just a PostgreSQL database and S3-compatible storage.

Download the binary from the [releases page](https://github.com/sendrec/sendrec/releases), set the required environment variables, and run:

```bash
export DATABASE_URL=postgres://sendrec:secret@localhost:5432/sendrec?sslmode=disable
export JWT_SECRET=$(openssl rand -hex 32)
export BASE_URL=https://videos.example.com
export S3_ENDPOINT=https://storage.example.com
export S3_ACCESS_KEY=your-key
export S3_SECRET_KEY=your-secret
export S3_BUCKET=recordings

./sendrec
```

Migrations run automatically on startup. See the [environment variables](#environment-variables) section for all configuration options.

To build from source:

```bash
git clone https://github.com/sendrec/sendrec.git
cd sendrec
make build   # builds frontend + Go binary
./sendrec
```

## Production setup

SendRec uses [Garage](https://garagehq.deuxfleurs.fr/) for S3-compatible object storage. Garage is lightweight (uses ~50 MB RAM), open source (AGPL-3.0), and purpose-built for self-hosted deployments. Any S3-compatible storage works — just change the `S3_ENDPOINT` and credentials.

First, create a `garage.toml` configuration file:

```toml
# garage.toml
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"

replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "<generate with: openssl rand -hex 32>"

[s3_api]
s3_region = "eu-central-1"
api_bind_addr = "[::]:3900"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "<generate with: openssl rand -base64 32>"
```

Then create a `garage-init.sh` script that sets up the bucket and credentials on first start. Garage generates its own key IDs (prefixed with `GK`), so the init script creates a key and writes the credentials to a shared volume that the app reads on startup. The init container uses a small Dockerfile (`Dockerfile.garage-init`) that copies the `garage` binary from the Garage image into Alpine:

```dockerfile
# Dockerfile.garage-init
FROM dxflrs/garage:v2.2.0 AS garage
FROM alpine:3.21
RUN apk add --no-cache aws-cli
COPY --from=garage /garage /usr/local/bin/garage
```

```bash
#!/bin/sh
set -e
S3_BUCKET="${S3_BUCKET:-recordings}"
GARAGE_KEYS_FILE="${GARAGE_KEYS_FILE:-/run/garage-keys/env}"

until garage status > /dev/null 2>&1; do sleep 1; done
NODE_ID=$(garage status 2>/dev/null | grep -oE '[a-f0-9]{16}' | head -1)
garage layout assign -z dc1 -c 1G "${NODE_ID}" 2>/dev/null || true
garage layout apply --version 1 2>/dev/null || true

KEY_INFO=$(garage key create sendrec-key 2>/dev/null || true)
if [ -z "${KEY_INFO}" ]; then
  KEY_INFO=$(garage key info sendrec-key 2>/dev/null || true)
fi
KEY_ID=$(echo "${KEY_INFO}" | grep -oE 'GK[a-f0-9]{24}' | head -1)
SECRET=$(echo "${KEY_INFO}" | grep "Secret key" | sed 's/.*: *//')

if [ -n "${KEY_ID}" ] && [ -n "${SECRET}" ]; then
  mkdir -p "$(dirname "${GARAGE_KEYS_FILE}")"
  printf 'S3_ACCESS_KEY=%s\nS3_SECRET_KEY=%s\n' "${KEY_ID}" "${SECRET}" > "${GARAGE_KEYS_FILE}"
else
  echo "ERROR: Could not extract key credentials"; exit 1
fi

garage bucket create "${S3_BUCKET}" 2>/dev/null || true
if [ -n "${KEY_ID}" ]; then
  garage bucket allow --read --write --owner "${S3_BUCKET}" --key "${KEY_ID}" 2>/dev/null || true
fi

# Set CORS via aws-cli (Garage v2.2.0 admin API silently ignores corsConfig)
apk add --no-cache aws-cli > /dev/null 2>&1 || true
export AWS_ACCESS_KEY_ID="${KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${SECRET}"
export AWS_DEFAULT_REGION="eu-central-1"
aws --endpoint-url http://127.0.0.1:3900 s3api put-bucket-cors --bucket "${S3_BUCKET}" --cors-configuration '{
  "CORSRules": [{
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "PUT", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3600
  }]
}' 2>/dev/null || true
```

```yaml
# docker-compose.yml
services:
  sendrec:
    image: ghcr.io/sendrec/sendrec:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - garage-keys:/run/garage-keys:ro
    environment:
      - DATABASE_URL=postgres://sendrec:secret@postgres:5432/sendrec?sslmode=disable
      - JWT_SECRET=change-me-to-a-long-random-string
      - BASE_URL=https://videos.example.com
      - S3_ENDPOINT=http://garage:3900
      - S3_PUBLIC_ENDPOINT=https://storage.example.com
      - S3_BUCKET=recordings
      - AWS_REQUEST_CHECKSUM_CALCULATION=when_required
      - AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
    depends_on:
      garage-init:
        condition: service_completed_successfully
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: sendrec
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: sendrec
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sendrec"]
      interval: 5s
      timeout: 5s
      retries: 5

  garage:
    image: dxflrs/garage:v2.2.0
    restart: unless-stopped
    volumes:
      - ./garage.toml:/etc/garage.toml:ro
      - s3-meta:/var/lib/garage/meta
      - s3-data:/var/lib/garage/data

  garage-init:
    build:
      context: .
      dockerfile: Dockerfile.garage-init
    network_mode: "service:garage"
    depends_on:
      garage:
        condition: service_started
    volumes:
      - ./garage.toml:/etc/garage.toml:ro
      - s3-meta:/var/lib/garage/meta:ro
      - garage-keys:/run/garage-keys
      - ./garage-init.sh:/garage-init.sh:ro
    environment:
      - S3_BUCKET=recordings
    entrypoint: ["/bin/sh", "/garage-init.sh"]

volumes:
  db-data:
  s3-meta:
  s3-data:
  garage-keys:
```

Put a reverse proxy (Caddy, nginx, Traefik) in front to handle TLS. The proxy should route your app domain to port 8080 and your storage domain to Garage port 3900.

## Environment variables

### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret for signing auth tokens. Must be set in production (app exits if empty when `BASE_URL` is HTTPS) |
| `BASE_URL` | Public URL of the app (e.g. `https://videos.example.com`). Used for CORS, share links, and cookies |

### Storage (S3-compatible)

| Variable | Description | Default |
|----------|-------------|---------|
| `S3_ENDPOINT` | S3-compatible API endpoint. For Garage in Docker, use the internal hostname (e.g. `http://garage:3900`) | `http://localhost:3900` |
| `S3_PUBLIC_ENDPOINT` | Public URL for the same S3 service, used to generate presigned URLs that browsers can reach. When Garage runs behind a reverse proxy, this should be the external URL (e.g. `https://storage.example.com`). If not set, `S3_ENDPOINT` is used — which works in dev but breaks in Docker where `S3_ENDPOINT` points to an internal hostname | — |
| `S3_BUCKET` | Bucket name for video storage | `recordings` |
| `S3_ACCESS_KEY` | S3 access key. When using Garage with the init script, this is auto-generated and passed via shared volume | — |
| `S3_SECRET_KEY` | S3 secret key. When using Garage with the init script, this is auto-generated and passed via shared volume | — |
| `S3_REGION` | S3 region. Must match the `s3_region` in your `garage.toml` | `eu-central-1` |
| `AWS_REQUEST_CHECKSUM_CALCULATION` | Set to `when_required` for S3-compatible storage providers | — |
| `AWS_RESPONSE_CHECKSUM_VALIDATION` | Set to `when_required` for S3-compatible storage providers | — |

### Limits

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_UPLOAD_BYTES` | Maximum upload size in bytes (applies to both recordings and file uploads) | `524288000` (500 MB) |
| `MAX_VIDEOS_PER_MONTH` | Maximum videos a user can create per month (recordings + uploads). Set to `0` for unlimited | `25` |
| `MAX_VIDEO_DURATION_SECONDS` | Maximum recording duration in seconds. Set to `0` for unlimited | `300` (5 min) |
| `MAX_PLAYLISTS` | Maximum playlists a free-tier user can create. Set to `0` for unlimited | `3` |

### API Documentation

| Variable | Description | Default |
|----------|-------------|---------|
| `API_DOCS_ENABLED` | Serve interactive API docs at `/api/docs` | `false` |

### Branding

| Variable | Description | Default |
|----------|-------------|---------|
| `BRANDING_ENABLED` | Allow users to customize watch page branding (logo, colors, footer) | `false` |

### Analytics (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `ANALYTICS_SCRIPT` | A `<script>` tag to inject on every watch page. Works with any analytics provider (Umami, Plausible, Matomo, etc.). The CSP nonce is added automatically. Example: `<script defer src="/script.js" data-website-id="xxx"></script>` | — |

### Nextcloud integration (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `ALLOWED_FRAME_ANCESTORS` | Space-separated list of domains allowed to embed SendRec in iframes (CSP `frame-ancestors`). Set to your Nextcloud URL for rich link previews. Example: `https://nextcloud.example.com` | `'self'` |

API keys for machine-to-machine access (used by the Nextcloud integration for video search) are managed per-user in **Settings > API Keys**. Each user generates their own keys — no server-side configuration needed.

### Transcription (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `TRANSCRIPTION_ENABLED` | Enable automatic video transcription | `true` |
| `WHISPER_MODEL_PATH` | Path to the whisper.cpp model file | `/models/ggml-small.bin` |

### AI Summaries (optional)

Generate automatic summaries and chapter markers for transcribed videos using any OpenAI-compatible API.

| Variable | Description | Default |
|----------|-------------|---------|
| `AI_ENABLED` | Enable AI summary generation after transcription | `false` |
| `AI_BASE_URL` | OpenAI-compatible API base URL | — |
| `AI_API_KEY` | API key for the AI provider | — |
| `AI_MODEL` | Model name to use | `mistral-small-latest` |
| `AI_TIMEOUT` | HTTP timeout for AI API requests. Applies to all providers. Increase for slower endpoints like local Ollama. Uses Go duration format (`60s`, `5m`, `10m`) | `60s` |

**Supported providers:** Any OpenAI-compatible API — Mistral AI, OpenAI, Ollama (local), and others.

Examples:
- **Mistral AI:** `AI_BASE_URL=https://api.mistral.ai`, `AI_API_KEY=your-key`, `AI_MODEL=mistral-small-latest`
- **OpenAI:** `AI_BASE_URL=https://api.openai.com`, `AI_API_KEY=your-key`, `AI_MODEL=gpt-4o-mini`
- **Ollama (local):** `AI_BASE_URL=http://ollama:11434`, `AI_API_KEY=` (empty), `AI_MODEL=llama3.2`, `AI_TIMEOUT=5m`

### Webhooks (optional)

Receive real-time event notifications via HTTP POST to any URL. Events include video created, ready, deleted, viewed, commented, milestone reached, and CTA clicked. Each request includes an `X-Webhook-Signature` header (HMAC-SHA256) for payload verification.

1. In SendRec **Settings > Webhooks**, enter your endpoint URL and click Save
2. A signing secret is auto-generated — copy it to verify signatures on your end
3. Click **Send test event** to verify delivery
4. Recent deliveries (last 50) are shown in Settings with status codes and response bodies

No server-side configuration is needed — each user configures their own webhook URL and secret in Settings.

### Slack notifications (optional)

Receive view and comment notifications in a Slack channel via incoming webhooks.

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and create a new app (From scratch)
2. Under **Incoming Webhooks**, activate and add a webhook to your channel
3. Copy the webhook URL
4. In SendRec **Settings > Slack Notifications**, paste the URL and click Save
5. Click **Send test message** to verify

No server-side configuration is needed — each user configures their own webhook URL in Settings.

### Billing (optional)

Enable subscription billing with [Creem](https://creem.io) (EU merchant of record). When configured, users see a billing section in Settings where they can upgrade to Pro. Without these variables, all users get the free tier and no billing UI is shown.

| Variable | Description |
|----------|-------------|
| `CREEM_API_KEY` | Creem API key. Test keys (`creem_test_` prefix) auto-route to `test-api.creem.io`; production keys use `api.creem.io` |
| `CREEM_PRO_PRODUCT_ID` | Creem product ID for the Pro plan. Must be a recurring/subscription product, not one-time |
| `CREEM_WEBHOOK_SECRET` | Signing secret for verifying Creem webhook payloads (HMAC-SHA256) |

**Creem webhook URL:** Configure `https://your-domain.com/api/webhooks/creem` in the Creem dashboard. Subscribe to all subscription events (`subscription.active`, `subscription.paid`, `subscription.canceled`, `subscription.expired`).

**Self-hosters without billing:** Skip these variables entirely. Control limits with `MAX_VIDEOS_PER_MONTH` and `MAX_VIDEO_DURATION_SECONDS` (set to `0` for unlimited).

### Email notifications (optional)

| Variable | Description |
|----------|-------------|
| `LISTMONK_BASE_URL` | Listmonk instance URL |
| `LISTMONK_USERNAME` | Listmonk API username |
| `LISTMONK_PASSWORD` | Listmonk API password |
| `LISTMONK_TEMPLATE_ID` | Template ID for share link emails |
| `LISTMONK_COMMENT_TEMPLATE_ID` | Template ID for new comment notifications |
| `LISTMONK_VIEW_TEMPLATE_ID` | Template ID for view notifications (instant and digest) |
| `LISTMONK_CONFIRM_TEMPLATE_ID` | Template ID for email confirmation on signup. Template variables: `{{ .Tx.Data.name }}`, `{{ .Tx.Data.confirmLink }}`. Confirmation emails bypass the allowlist and are always sent |
| `EMAIL_ALLOWLIST` | Comma-separated list of allowed recipient domains (`@example.com`) and addresses (`alice@example.com`). When set, emails are only sent to matching recipients (except confirmation emails). Useful for staging/preview environments |

## S3_PUBLIC_ENDPOINT explained

Video recordings and file uploads use presigned S3 URLs. The app generates these URLs using `S3_PUBLIC_ENDPOINT` so the browser can upload directly to storage (MP4, WebM, and MOV files are supported).

- **In development:** Garage is exposed on `localhost:3900`, no `S3_PUBLIC_ENDPOINT` needed
- **In production:** Garage typically runs behind a reverse proxy. `S3_ENDPOINT` points to the internal Docker hostname (`http://garage:3900`), but browsers can't reach that. Set `S3_PUBLIC_ENDPOINT` to the external URL (e.g. `https://storage.example.com`) so presigned URLs work

## Using other S3-compatible storage

SendRec works with any S3-compatible storage provider. Just set the `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, and `S3_BUCKET` environment variables. Examples:

- **Hetzner Object Storage:** `S3_ENDPOINT=https://fsn1.your-objectstorage.com`
- **Cloudflare R2:** `S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com`
- **Backblaze B2:** `S3_ENDPOINT=https://s3.eu-central-003.backblazeb2.com`
- **AWS S3:** `S3_ENDPOINT=https://s3.eu-central-1.amazonaws.com`

When using a managed S3 provider, you don't need the `garage` and `garage-init` services in your Docker Compose file — just the `sendrec` and `postgres` services.

## Removing usage limits

By default, SendRec enforces free tier limits (25 videos/month, 5 min max duration). For self-hosted instances, disable them:

```yaml
environment:
  - MAX_VIDEOS_PER_MONTH=0        # 0 = unlimited
  - MAX_VIDEO_DURATION_SECONDS=0  # 0 = unlimited
```

## Enabling transcription

The Docker image includes `whisper-cli`. To enable transcription, download a whisper model and mount it:

```yaml
services:
  sendrec:
    volumes:
      - ./models:/models:ro
    environment:
      - TRANSCRIPTION_ENABLED=true
      - WHISPER_MODEL_PATH=/models/ggml-small.bin
```

Download the model (~466 MB):
```bash
mkdir -p models
curl -L -o models/ggml-small.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
```

Without the model, transcription is silently skipped and everything else works normally.

## Updating

To update a running instance to the latest version:

```bash
docker compose pull sendrec
docker compose up -d sendrec
```

Database migrations run automatically on startup — no manual steps needed. The app checks for pending migrations and applies them before accepting requests.

To pin a specific version instead of `latest`:

```yaml
services:
  sendrec:
    image: ghcr.io/sendrec/sendrec:v1.48.0
```

Check the [releases page](https://github.com/sendrec/sendrec/releases) for available versions and changelogs.

**Backup first.** Before major updates, back up your PostgreSQL database:

```bash
docker compose exec postgres pg_dump -U sendrec sendrec > backup.sql
```

## Reverse proxy example (Caddy)

```
videos.example.com {
    reverse_proxy sendrec:8080
}

storage.example.com {
    reverse_proxy garage:3900
}
```
