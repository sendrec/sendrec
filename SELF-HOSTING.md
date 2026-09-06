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

## Kubernetes with Helm

SendRec includes a Helm chart at `helm/sendrec` for Kubernetes deployments.

### Prerequisites

- Kubernetes cluster with an ingress controller (Traefik, nginx, etc.)
- Helm 3
- PostgreSQL database
- S3-compatible object storage
- **Start with 1 GiB for the app container and measure your workload** using [Sizing the container](#sizing-the-container). ffmpeg is a child process in the HTTP server's container: memory pressure can take down live traffic as well as the edit. High-resolution sources, local transcription and concurrent jobs can require more.

### 1. Create a values file

Create a deployment-specific values file (for example `values-prod.yaml`) and keep your environment settings there:

```yaml
sendrec:
  env:
    baseUrl: "https://sendrec.yourdomain.com"
    s3Endpoint: "https://s3.amazonaws.com"
    s3PublicEndpoint: "https://s3.amazonaws.com"
    s3Bucket: "recordings"
    s3Region: "eu-central-1"
    transcriptionEnabled: "false"
    googleAuthAllowedDomains: "example.com"

  secrets:
    databaseUrl: "postgres://sendrec:secret@postgres:5432/sendrec"
    jwtSecret: "change-me-to-a-long-random-string"
    s3AccessKey: "your-access-key"
    s3SecretKey: "your-secret-key"
```

### 2. Install the chart

```bash
helm install sendrec ./helm/sendrec \
  --namespace sendrec \
  --create-namespace \
  -f values-prod.yaml
```

### 3. Verify rollout

```bash
kubectl -n sendrec get pods
kubectl -n sendrec get svc
```

### 4. Upgrade after changes

```bash
helm upgrade sendrec ./helm/sendrec \
  --namespace sendrec \
  -f values-prod.yaml
```

### Optional: preview rendered manifests

```bash
helm template sendrec ./helm/sendrec -f values-prod.yaml
```

### Helm notes

- Non-secret environment variables are configured under `sendrec.env` (camelCase keys).
- Sensitive values are configured under `sendrec.secrets`.
- Transcription model resources (init container, model volume, and optional PVC) are controlled by `sendrec.env.transcriptionEnabled`.
- To persist the Whisper model, set `sendrec.transcription.type: volume`.

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

After starting Garage, initialize it by running a few commands inside the container:

```bash
# Start Garage
docker compose up -d garage

# Initialize the cluster layout
NODE_ID=$(docker compose exec garage /garage status 2>/dev/null | grep -oE '[a-f0-9]{16}' | head -1)
docker compose exec garage /garage layout assign -z dc1 -c 1G "$NODE_ID"
docker compose exec garage /garage layout apply --version 1

# Create an API key and bucket
docker compose exec garage /garage key create sendrec-key
docker compose exec garage /garage bucket create recordings
docker compose exec garage /garage bucket allow --read --write --owner recordings --key sendrec-key
```

Copy the `Key ID` (starts with `GK`) and `Secret key` from the output — you'll need them for `S3_ACCESS_KEY` and `S3_SECRET_KEY` below.

```yaml
# docker-compose.yml
services:
  sendrec:
    image: ghcr.io/sendrec/sendrec:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://sendrec:secret@postgres:5432/sendrec?sslmode=disable
      - JWT_SECRET=change-me-to-a-long-random-string
      - BASE_URL=https://videos.example.com
      - S3_ENDPOINT=http://garage:3900
      - S3_PUBLIC_ENDPOINT=https://storage.example.com
      - S3_BUCKET=recordings
      - S3_ACCESS_KEY=GKxxxxxxxxxxxxxxxxxxxxxxxx
      - S3_SECRET_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      - AWS_REQUEST_CHECKSUM_CALCULATION=when_required
      - AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: sendrec
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: sendrec
    volumes:
      - db-data:/var/lib/postgresql
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

volumes:
  db-data:
  s3-meta:
  s3-data:
```

Put a reverse proxy (Caddy, nginx, Traefik) in front to handle TLS. The proxy should route your app domain to port 8080 and your storage domain to Garage port 3900.

### Rate limiting behind a proxy (`TRUSTED_PROXY`)

Per-IP rate limiting keys on the connection's source address. Behind a reverse proxy every request arrives from the *proxy's* address, so the setting matters:

- **`TRUSTED_PROXY` unset or `false`, behind a proxy** — all traffic looks like it comes from the proxy IP, so every user shares a single rate-limit bucket. One busy client trips the limit for everyone — a self-inflicted denial of service.
- **`TRUSTED_PROXY=true`** — the app believes the last `X-Forwarded-For` entry and limits per real client IP.

Set `TRUSTED_PROXY=true` **only** when an edge proxy you control always sets `X-Forwarded-For` (Caddy, nginx, and Traefik do by default). If the app is exposed to the internet directly, leave it `false`: otherwise clients forge the header and evade rate limiting entirely.

## Sizing the container

ffmpeg runs as a child process in the HTTP server's container. Encoder settings do not impose a container memory ceiling: decoders still hold full-resolution input frames, even when the output is scaled down.

`MAX_CONCURRENT_ENCODES` (default `1`) caps simultaneous encodes per app process. Extra edits queue. It does not cover local transcription, probes, thumbnails, or downloads/uploads. Raise concurrency and the memory allowance together.

The chart now requests **1Gi**, without a default memory limit. This replaces the 512Mi estimate from a synthetic 1080p encode. On 2026-09-05, staging image `6219d0a` (after #208) held **837.3 MiB of anon** while removing 200 ranges from a 3242×2626, 1000 fps recording; idle was **6.64 MiB**, shmem was zero, and only one encode ran. With N=1 and 20% headroom, that requires **1004.8 MiB**, exceeding 512Mi by **492.8 MiB**. The job timed out after ten minutes, so this is a measured lower bound, not a completed-job ceiling. Remove-segments now scales its output to fit 1920×1080 at 60 fps; remeasure after upgrading. The 1Gi default is a starting reservation, not a guarantee for every source.

With these fixes, the same 200-cut edit on a copy completed in **367.3 seconds**. A disposable container using staging's FFmpeg 6.1.2 runtime and the patched binary (SHA-256 `534ea9aaeec27bd31464f1ca0c7c2473a761648dea84f70c71640bfc2400c5b8`, image created 2026-09-05 21:36 UTC) measured:

| Measurement | Result |
| --- | --- |
| Idle anon | 5,836,800 bytes (5.57 MiB) |
| Peak anon | 381,378,560 bytes (363.71 MiB) |
| Within 5% of peak | 362.0 seconds, 363 one-second samples |
| Shmem / swap / OOMs / restarts | Zero |
| N=1 reservation: `(idle + (peak − idle)) × 1.2` | 457,654,272 bytes (436.45 MiB) |
| Margin below 512Mi / 1Gi | 75.55 MiB / 587.55 MiB |

This was the largest-resolution staging recording: 3242×2626, 43.167 seconds, 43,167 video frames, with audio. The completed output was 1334×1080 at 60 fps; video lasted 37.183 seconds and audio 37.172 seconds. The host exposed four CPUs, `/tmp` was disk-backed, and neither the container nor its ancestors imposed a memory limit. Only the app, its one edit subprocess at a time, and short-lived sampler commands ran in the measured cgroup; transcription was disabled and no other edit was active. The source and final-output probes ran separately, outside the measured container. The copied recording, container and sampler resources were removed; the original recording was unchanged.

**512Mi covers this completed single-job run, not every accepted input or worker combination.** The 1Gi default remains a conservative starting reservation; reducing it requires a measurement on the image and workload you actually deploy. Decoding larger sources and enabling other workers can still exceed it.

**Image matters:** the chart still defaults to `v1.90.5`, which predates #208, the concurrency gate and these edit fixes. That image ignores `MAX_CONCURRENT_ENCODES`; do not assume N=1 just because the chart sets it. Updating the chart alone does not install the fixed application. Select a reviewed image containing these fixes once released, then measure it; neither measurement certifies the older default image.

### Measuring your own floor

Use staging or a disposable copy of a recording. Confirm the container serves the intended hostname and its image contains the changes being tested; a later image creation date alone does not prove which code it contains. Wait for unrelated work to finish. These commands require Bash on the host and cgroup v2 in the container:

```bash
C=$(docker inspect --format '{{.Id}}' sendrec)  # replace sendrec with your container
docker inspect --format 'name={{.Name}} image={{.Image}} restarts={{.RestartCount}}' "$C"
docker image inspect --format 'created={{.Created}} labels={{json .Config.Labels}}' \
  "$(docker inspect --format '{{.Image}}' "$C")"
docker exec "$C" sh -c 'cat /proc/1/cgroup /proc/self/cgroup; cat /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory.swap.current /sys/fs/cgroup/memory.events; df -T /tmp'

# Require a private cgroup namespace with its root mounted here. Matching
# /proc paths alone can still mean memory.stat belongs to the host root.
# Do not certify a run with swapping, OOMs, or a restart.
sample() {
  docker exec "$C" sh -c '
    test "$(cat /proc/1/cgroup)" = "0::/" &&
    test "$(cat /proc/self/cgroup)" = "0::/" &&
    awk '\''$4=="/" && $5=="/sys/fs/cgroup" && / - cgroup2 / {ok=1}
      END{exit !ok}'\'' /proc/self/mountinfo || {
      echo "Unsupported cgroup scope: resolve the app cgroup before sampling" >&2
      exit 1
    }
    awk '\''$1=="anon"{a=$2} $1=="shmem"{s=$2}
      END{printf "bytes %.0f %.0f %.0f\n", a, s, a+s}'\'' /sys/fs/cgroup/memory.stat &&
    ps -o pid,ppid,comm'
}
sample  # idle baseline: bytes anon shmem sum; check the process list

# Trigger the heaviest edit on a COPY in another terminal. Record whether it
# completes. The loop is on the HOST, bounded to 900 samples, with no remote
# sleep/loop to survive Ctrl-C. Every exec is a single short-lived snapshot.
for ((i=0; i<900; i++)); do
  date -u +%FT%TZ
  sample || { echo "Sampling failed; discard this run" >&2; break; }
  sleep 1
done | awk '$1=="bytes" {if ($4>m) m=$4;
  printf "anon=%.2f MiB shmem=%.2f MiB total=%.2f MiB peak=%.2f MiB\n", $2/1048576, $3/1048576, $4/1048576, m/1048576;
  next} {print} {fflush()}'

# After stopping: verify no sampler or edit remains inside the container.
docker exec "$C" sh -c 'ps -o pid,ppid,comm; ls -la /tmp; cat /sys/fs/cgroup/memory.events'
docker inspect --format 'restarts={{.RestartCount}}' "$C"
```

If the scope check fails, stop: on a host-cgroup-namespace container, resolve the app's exact cgroup path and mount before adapting the sampler. Do not remove the guard and read the host root instead. Also check ancestor limits from the host; they may not be visible inside a private namespace.

The samples must rise when ffmpeg starts and fall when it exits. Keep timestamps and PIDs: a second job taking the slot, another process, or an unexpected shared cgroup changes what the peak means. Report the peak and time held within 5% of it. A failed/timed-out encode supplies only a lower bound; repeat through successful completion before certifying a size. Compare each stream's output duration as well as the job status.

Use **(idle + N × (peak − idle)) × 1.2** as a starting memory reservation, with idle and peak both measured as anon + shmem, and N taken from the running app's `MAX_CONCURRENT_ENCODES` setting/startup log. At N=1 this is peak × 1.2. Test actual concurrency before relying on the extrapolation; queued downloads, thumbnails and transcription need their own allowance.

**This is not a formula for a hard limit.** `mem_limit` (Compose) and `--memory` include kernel memory and page cache too. Measure their behavior under the proposed limit with headroom for larger inputs; a request/reservation does not enforce a limit. Docker/Coolify does not perform Kubernetes request-based scheduling.

Read **anon + shmem**, not `docker stats`, `memory.current` or `memory.peak`, for this resident working-set estimate. Those totals include reclaimable disk page cache from temporary files, and a constrained total can merely report the limit. Anonymous memory and tmpfs/shmem cannot be reclaimed without swapping; the sampler includes shmem automatically. They are not the whole charged footprint: kernel memory and actively used file-backed pages can also matter. One-second snapshots can miss brief spikes. If the container or an ancestor has a tight limit, or `memory.swap.current` is nonzero, do not treat its resident peak as unconstrained demand.

Re-measure when you change resolution limits, enable transcription or noise reduction, or raise the concurrency limit — each moves the floor.

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
| `S3_ACCESS_KEY` | S3 access key | — |
| `S3_SECRET_KEY` | S3 secret key | — |
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

### Registration

| Variable | Description | Default |
|----------|-------------|---------|
| `REGISTRATION_ENABLED` | Allow new user signups. Set to `false` to disable the registration form and API endpoint | `true` |
| `PLAN_BADGE_ENABLED` | Show the plan badge ("Free"/"Pro"/"Business") next to the SendRec logo. Off by default for self-hosters; set to `true` to surface upgrade tiers | `false` |

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
| `TRANSCRIPTION_ENABLED` | Enable automatic video transcription | `false` |
| `TRANSCRIPTION_PROVIDER` | `local`, `openai`, or `deepgram` | `local` |
| `WHISPER_MODEL_PATH` | Path to the whisper.cpp model file (only for `local`) | `/models/ggml-small.bin` |
| `TRANSCRIPTION_API_URL` | Base URL for OpenAI-compatible providers (omit for `https://api.openai.com`) | — |
| `TRANSCRIPTION_API_KEY` | API key for cloud providers | — |
| `TRANSCRIPTION_MODEL` | Model name; defaults `whisper-1` (openai), `nova-3` (deepgram) | — |
| `TRANSCRIPTION_TIMEOUT_SECONDS` | HTTP timeout for cloud calls (seconds) | `300` |

**Provider notes:**
- `local` — runs `whisper-cli` on the app container; CPU-bound. Best for full privacy or offline deployments.
- `openai` — POSTs audio to any OpenAI-compatible `/v1/audio/transcriptions` endpoint. Works with OpenAI Whisper, Groq Whisper, Scaleway Speech-to-Text, self-hosted Faster-Whisper, etc.
- `deepgram` — POSTs audio to `https://api.deepgram.com/v1/listen`. US-hosted.

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

SendRec supports two email backends: [Listmonk](https://listmonk.app) (preferred when configured) and direct SMTP. If neither is configured, **email confirmation is automatically skipped** at signup so new users can sign in immediately.

#### Listmonk

Only the base URL and credentials are required — all template IDs are optional. When a template ID is not set, a plain HTML fallback email is sent automatically.

| Variable | Description |
|----------|-------------|
| `LISTMONK_URL` | Listmonk instance URL |
| `LISTMONK_USER` | Listmonk API username |
| `LISTMONK_PASSWORD` | Listmonk API password |
| `LISTMONK_TEMPLATE_ID` | Template ID for share link emails (optional — fallback sends plain email) |
| `LISTMONK_COMMENT_TEMPLATE_ID` | Template ID for new comment notifications (optional) |
| `LISTMONK_VIEW_TEMPLATE_ID` | Template ID for view notifications and weekly digest (optional) |
| `LISTMONK_CONFIRM_TEMPLATE_ID` | Template ID for email confirmation on signup (optional). Template variables: `{{ .Tx.Data.name }}`, `{{ .Tx.Data.confirmLink }}`. Confirmation emails bypass the allowlist |
| `LISTMONK_WELCOME_TEMPLATE_ID` | Template ID for the welcome email after email confirmation (optional). Template variables: `{{ .Tx.Data.name }}`, `{{ .Tx.Data.dashboardURL }}`. Bypasses the allowlist |
| `LISTMONK_ONBOARDING_DAY2_TEMPLATE_ID` | Template ID for day 2 onboarding email (optional). Bypasses the allowlist |
| `LISTMONK_ONBOARDING_DAY7_TEMPLATE_ID` | Template ID for day 7 onboarding email (optional). Bypasses the allowlist |
| `LISTMONK_ORG_INVITE_TEMPLATE_ID` | Template ID for workspace invitation emails (optional). Template variables: `{{ .Tx.Data.orgName }}`, `{{ .Tx.Data.inviterName }}`, `{{ .Tx.Data.acceptLink }}`. Bypasses the allowlist |
| `LISTMONK_RETENTION_WARNING_TEMPLATE_ID` | Template ID for data retention warning emails (optional). Template variables: `{{ .Tx.Data.videos }}`, `{{ .Tx.Data.expiryDate }}`. Bypasses the allowlist |
| `EMAIL_ALLOWLIST` | Comma-separated list of allowed recipient domains (`@example.com`) and addresses (`alice@example.com`). When set, emails are only sent to matching recipients (except confirmation, welcome, onboarding, invite, and retention emails). Useful for staging/preview environments |

#### SMTP (used when Listmonk is not set)

Set `SMTP_HOST` to enable a direct SMTP relay (Gmail, SES, Postmark, your own server). All transactional emails use the same plain HTML bodies as the Listmonk fallback.

| Variable | Description |
|----------|-------------|
| `SMTP_HOST` | SMTP server hostname (e.g. `smtp.gmail.com`) |
| `SMTP_PORT` | SMTP server port (default `587`) |
| `SMTP_USERNAME` | Auth username (omit for unauthenticated relays) |
| `SMTP_PASSWORD` | Auth password / app password |
| `SMTP_TLS` | `starttls` (default — fails if server does not advertise STARTTLS), `tls` (implicit TLS, use port 465), `auto` (try STARTTLS, fall back to plaintext — **plaintext-only relays must be unauthenticated**: when `SMTP_USERNAME` is set, the Go stdlib refuses PLAIN auth on a non-TLS connection unless the host is `localhost`, so configure either `starttls`/`tls` for authenticated relays or omit credentials entirely), or `none` (plaintext, same auth restriction). Any other value (typo, whitespace, unsupported keyword) is coerced to `starttls` with a startup warning so a misconfigured `start_tls` cannot silently downgrade to plaintext. |
| `EMAIL_FROM_ADDRESS` | `From:` address used for both Listmonk and SMTP (default `noreply@sendrec.eu`) |

#### Sendmail (opt-in fallback)

If you run a local MTA (postfix, exim, etc.) and want SendRec to use the `sendmail(8)` binary, set:

| Variable | Description |
|----------|-------------|
| `EMAIL_USE_SENDMAIL` | `true` to enable. Off by default. The binary must be on `$PATH` — if it isn't, the deployment is treated as having no email backend (registrations auto-verify). |

When enabled, sendmail is also used as a fallback if Listmonk is set but the request fails.

**No email backend at all:** Leave `LISTMONK_URL`, `SMTP_HOST`, and `EMAIL_USE_SENDMAIL` unset. New accounts skip email confirmation and can sign in immediately. Existing unverified users from a previous configuration will be blocked from login until `email_verified` is flipped manually:

```sql
UPDATE users SET email_verified = true WHERE email_verified = false;
```

**Upgrade note:** Pre-1.85 builds silently used `sendmail(8)` whenever Listmonk wasn't configured. With this release, sendmail is opt-in via `EMAIL_USE_SENDMAIL=true`. If your deployment relied on the implicit sendmail fallback, set the variable explicitly.

### Social login / SSO (optional)

SendRec supports Google, Microsoft, and GitHub as social login providers via OAuth 2.0 / OIDC. Set the client ID and secret pair for each provider you want to enable; providers without credentials are simply not advertised on the login screen.

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 client secret |
| `MICROSOFT_CLIENT_ID` | Microsoft (Entra ID) OAuth 2.0 client ID |
| `MICROSOFT_CLIENT_SECRET` | Microsoft (Entra ID) OAuth 2.0 client secret |
| `GITHUB_SSO_CLIENT_ID` | GitHub OAuth App client ID |
| `GITHUB_SSO_CLIENT_SECRET` | GitHub OAuth App client secret |
| `GOOGLE_AUTH_ALLOWED_DOMAINS` | Comma-separated list of email domains permitted to sign in with Google (e.g. `example.com,partner.com`). When set, only verified Google accounts whose email domain matches one of these entries can authenticate. Exact match only — `example.com` does **not** imply `mail.example.com`. Leave unset to allow any Google account |

Configure the following authorized redirect URIs in each provider's console (replace `<your-base-url>` with your `BASE_URL`, no trailing slash):

| Provider | Redirect URI |
|----------|--------------|
| Google | `https://<your-base-url>/api/auth/sso/google/callback` |
| Microsoft | `https://<your-base-url>/api/auth/sso/microsoft/callback` |
| GitHub | `https://<your-base-url>/api/auth/sso/github/callback` |

For SAML / per-organization OIDC enforcement, see the in-app workspace settings.

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

When using a managed S3 provider, you don't need the `garage` service in your Docker Compose file — just the `sendrec` and `postgres` services.

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
    image: ghcr.io/sendrec/sendrec:v1.70.0
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
