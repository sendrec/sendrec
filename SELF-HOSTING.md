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

## Production setup

```yaml
# docker-compose.yml
services:
  sendrec:
    image: ghcr.io/sendrec/sendrec:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://sendrec:secret@postgres:5432/sendrec?sslmode=disable
      - JWT_SECRET=change-me-to-a-long-random-string
      - BASE_URL=https://videos.example.com
      - S3_ENDPOINT=http://minio:9000
      - S3_PUBLIC_ENDPOINT=https://storage.example.com
      - S3_BUCKET=recordings
      - S3_ACCESS_KEY=minioadmin
      - S3_SECRET_KEY=minioadmin
    depends_on:
      - postgres
      - minio

  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: sendrec
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: sendrec
    volumes:
      - db-data:/var/lib/postgresql/data

  minio:
    image: quay.io/minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - s3-data:/data

volumes:
  db-data:
  s3-data:
```

Put a reverse proxy (Caddy, nginx, Traefik) in front to handle TLS. The proxy should route your app domain to port 8080 and your storage domain to MinIO port 9000.

## Environment variables

### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret for signing auth tokens. Must be set in production (app exits if empty when `BASE_URL` is HTTPS) |
| `BASE_URL` | Public URL of the app (e.g. `https://videos.example.com`). Used for CORS, share links, and cookies |

### Storage (S3 / MinIO)

| Variable | Description | Default |
|----------|-------------|---------|
| `S3_ENDPOINT` | S3-compatible API endpoint. For MinIO in Docker, use the internal hostname (e.g. `http://minio:9000`) | `http://localhost:9000` |
| `S3_PUBLIC_ENDPOINT` | Public URL for the same S3 service, used to generate presigned URLs that browsers can reach. When MinIO runs behind a reverse proxy, this should be the external URL (e.g. `https://storage.example.com`). If not set, `S3_ENDPOINT` is used — which works in dev but breaks in Docker where `S3_ENDPOINT` points to an internal hostname | — |
| `S3_BUCKET` | Bucket name for video storage | `recordings` |
| `S3_ACCESS_KEY` | S3 access key | — |
| `S3_SECRET_KEY` | S3 secret key | — |
| `S3_REGION` | S3 region | `eu-central-1` |

### Limits

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_UPLOAD_BYTES` | Maximum upload size in bytes (applies to both recordings and file uploads) | `524288000` (500 MB) |
| `MAX_VIDEOS_PER_MONTH` | Maximum videos a user can create per month (recordings + uploads). Set to `0` for unlimited | `25` |
| `MAX_VIDEO_DURATION_SECONDS` | Maximum recording duration in seconds. Set to `0` for unlimited | `300` (5 min) |

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

### Transcription (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `TRANSCRIPTION_ENABLED` | Enable automatic video transcription | `true` |
| `WHISPER_MODEL_PATH` | Path to the whisper.cpp model file | `/models/ggml-small.bin` |

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

- **In development:** MinIO is exposed on `localhost:9000`, no `S3_PUBLIC_ENDPOINT` needed
- **In production:** MinIO typically runs behind a reverse proxy. `S3_ENDPOINT` points to the internal Docker hostname (`http://minio:9000`), but browsers can't reach that. Set `S3_PUBLIC_ENDPOINT` to the external URL (e.g. `https://storage.example.com`) so presigned URLs work

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
    image: ghcr.io/sendrec/sendrec:v1.25.0
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
    reverse_proxy minio:9000
}
```
