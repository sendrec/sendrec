# SendRec

The async video platform built for Europe. Open source, GDPR native, EU hosted.

![SendRec — The async video platform built for Europe](.github/screenshots/landing-hero.png)

## What is SendRec?

SendRec is an open-source alternative to Loom for teams that need their data to stay in the EU. Record your screen, share videos with your team, and keep full control of your data.

- **EU hosted** — all data stored on European servers, never leaves the EU
- **GDPR native** — privacy-first by design, not bolted on after the fact
- **Open source** — AGPLv3 licensed, self-host or use our managed platform
- **No US cloud dependency** — no data transfers to the US, no Schrems II risk
- **Automatic transcription** — videos are transcribed with whisper.cpp, displayed as subtitles and a clickable transcript panel

## Quick Start

```bash
git clone https://github.com/sendrec/sendrec.git
cd sendrec
cp .env.example .env
docker compose -f docker-compose.dev.yml up --build
```

Open http://localhost:8080, register an account, and start recording.

## Development

**Prerequisites:** Go 1.25+, Node 24+, pnpm, Docker

```bash
# Run the full stack with Docker
make docker-up

# Or run services separately:
make dev-web    # Frontend dev server (port 5173, proxies API to 8080)
make run        # Go server (requires DATABASE_URL, S3 env vars)

# Build everything
make build

# Run tests
make test
```

## Tech Stack

- **Frontend:** React 19, TypeScript 5.9, Vite 7
- **Backend:** Go (single binary, chi router)
- **Database:** PostgreSQL 18
- **Storage:** S3-compatible object storage (MinIO for dev, Hetzner Object Storage for prod)
- **Transcription:** [whisper.cpp](https://github.com/ggerganov/whisper.cpp) (optional, runs server-side)
- **Deployment:** Docker Compose

### Environment variables

#### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret for signing auth tokens. Must be set in production (app exits if empty when `BASE_URL` is HTTPS) |
| `BASE_URL` | Public URL of the app (e.g. `https://videos.example.com`). Used for CORS, share links, and cookies |

#### Storage (S3 / MinIO)

| Variable | Description | Default |
|----------|-------------|---------|
| `S3_ENDPOINT` | S3-compatible API endpoint. For MinIO in Docker, use the internal hostname (e.g. `http://minio:9000`) | `http://localhost:9000` |
| `S3_PUBLIC_ENDPOINT` | Public URL for the same S3 service, used to generate presigned URLs that browsers can reach. When MinIO runs behind a reverse proxy, this should be the external URL (e.g. `https://storage.example.com`). If not set, `S3_ENDPOINT` is used — which works in dev but breaks in Docker where `S3_ENDPOINT` points to an internal hostname | — |
| `S3_BUCKET` | Bucket name for video storage | `recordings` |
| `S3_ACCESS_KEY` | S3 access key | — |
| `S3_SECRET_KEY` | S3 secret key | — |
| `S3_REGION` | S3 region | `eu-central-1` |

#### Limits

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_UPLOAD_BYTES` | Maximum upload size in bytes | `524288000` (500 MB) |
| `MAX_VIDEOS_PER_MONTH` | Maximum videos a user can record per month. Set to `0` for unlimited | `25` |
| `MAX_VIDEO_DURATION_SECONDS` | Maximum recording duration in seconds. Set to `0` for unlimited | `300` (5 min) |

#### Transcription (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `TRANSCRIPTION_ENABLED` | Enable automatic video transcription | `true` |
| `WHISPER_MODEL_PATH` | Path to the whisper.cpp model file | `/models/ggml-small.bin` |

Transcription requires the whisper model to be available at `WHISPER_MODEL_PATH`. If the model file or `whisper-cli` binary is missing, transcription is silently skipped.

#### Email notifications (optional)

| Variable | Description |
|----------|-------------|
| `LISTMONK_BASE_URL` | Listmonk instance URL |
| `LISTMONK_USERNAME` | Listmonk API username |
| `LISTMONK_PASSWORD` | Listmonk API password |
| `LISTMONK_TEMPLATE_ID` | Template ID for share link emails |
| `LISTMONK_COMMENT_TEMPLATE_ID` | Template ID for new comment notifications |

## API Documentation

Interactive API reference is available at [`/api/docs`](https://app.sendrec.eu/api/docs) (powered by [Scalar](https://github.com/scalar/scalar)). The raw OpenAPI 3.0 spec is at [`/api/docs/openapi.yaml`](https://app.sendrec.eu/api/docs/openapi.yaml).

## Architecture

Single Go binary that:
- Serves the React SPA (embedded at build time)
- Handles REST API requests (`/api/*`)
- Serves interactive API documentation (`/api/docs`)
- Renders server-side watch pages with OpenGraph tags (`/watch/:token`)
- Runs database migrations on startup

Video recordings happen entirely in the browser using `getDisplayMedia` + `MediaRecorder`. Files upload directly to S3 via presigned URLs — the server never touches video bytes.

After upload, the server generates a thumbnail with ffmpeg and transcribes the audio with [whisper.cpp](https://github.com/ggerganov/whisper.cpp). Transcripts are stored as VTT subtitles and a clickable segment panel on the watch page. Transcription is optional — if the whisper model is not available, it is silently skipped.

## Self-Hosting

SendRec is designed to run on a single server with Docker Compose. A small VPS (2 vCPU, 4 GB RAM) is enough.

### Minimal setup

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

### S3_PUBLIC_ENDPOINT explained

Video uploads and downloads use presigned S3 URLs. The app generates these URLs using `S3_PUBLIC_ENDPOINT` so the browser can reach the storage service directly.

- **In development:** MinIO is exposed on `localhost:9000`, no `S3_PUBLIC_ENDPOINT` needed
- **In production:** MinIO typically runs behind a reverse proxy. `S3_ENDPOINT` points to the internal Docker hostname (`http://minio:9000`), but browsers can't reach that. Set `S3_PUBLIC_ENDPOINT` to the external URL (e.g. `https://storage.example.com`) so presigned URLs work

### Removing usage limits

By default, SendRec enforces free tier limits (25 videos/month, 5 min max duration). For self-hosted instances, disable them:

```yaml
environment:
  - MAX_VIDEOS_PER_MONTH=0        # 0 = unlimited
  - MAX_VIDEO_DURATION_SECONDS=0  # 0 = unlimited
```

### Enabling transcription

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

## Deployment

Deployments are automated via GitHub Actions. Three environments are available:

| Environment | URL | Trigger |
|-------------|-----|---------|
| **Preview** | `pr-{N}.app.sendrec.eu` | PR opened/updated (write-access authors only, max 3 concurrent) |
| **Staging** | `staging.app.sendrec.eu` | Push to `main` |
| **Production** | `app.sendrec.eu` | Push a git tag (`v*`) |

### Deploying to production

1. Merge your PR to `main` — staging deploys automatically
2. Verify on `staging.app.sendrec.eu`
3. Tag and push:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

Preview environments are cleaned up automatically when the PR is closed.

## License

SendRec is licensed under the [GNU Affero General Public License v3.0](LICENSE).

## Links

- **Website:** [sendrec.eu](https://sendrec.eu)
- **API docs:** [app.sendrec.eu/api/docs](https://app.sendrec.eu/api/docs)
- **Blog:** [sendrec.eu/blog](https://sendrec.eu/blog)
- **Email:** hello@sendrec.eu
