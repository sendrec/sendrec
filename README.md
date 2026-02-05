# SendRec

The async video platform built for Europe. Open source, GDPR native, EU hosted.

## What is SendRec?

SendRec is an open-source alternative to Loom for teams that need their data to stay in the EU. Record your screen, share videos with your team, and keep full control of your data.

- **EU hosted** — all data stored on European servers, never leaves the EU
- **GDPR native** — privacy-first by design, not bolted on after the fact
- **Open source** — AGPLv3 licensed, self-host or use our managed platform
- **No US cloud dependency** — no data transfers to the US, no Schrems II risk

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
- **Deployment:** Docker Compose

### Key environment variables
- `DATABASE_URL` – required
- `JWT_SECRET` – required
- `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION` (defaults to EU region)
- `BASE_URL` – used for CORS and share links
- `MAX_UPLOAD_BYTES` – max allowed upload size (bytes), defaults to 500MB

## Architecture

Single Go binary that:
- Serves the React SPA (embedded at build time)
- Handles REST API requests (`/api/*`)
- Renders server-side watch pages with OpenGraph tags (`/watch/:token`)
- Runs database migrations on startup

Video recordings happen entirely in the browser using `getDisplayMedia` + `MediaRecorder`. Files upload directly to S3 via presigned URLs — the server never touches video bytes.

## License

SendRec is licensed under the [GNU Affero General Public License v3.0](LICENSE).

## Links

- **Website:** [sendrec.eu](https://sendrec.eu)
- **Blog:** [sendrec.eu/blog](https://sendrec.eu/blog)
- **Email:** hello@sendrec.eu
