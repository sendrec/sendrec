# Contributing to SendRec

Thanks for your interest in contributing. This guide covers everything you need to get started.

## Getting Started

### Prerequisites

- Go 1.25+
- Node 24+ with pnpm
- Docker and Docker Compose

### Local Development

```bash
git clone https://github.com/sendrec/sendrec.git
cd sendrec
cp .env.example .env
```

**Option A: Full stack with Docker** (recommended for first-time setup)

```bash
make docker-up
# App runs at http://localhost:8080
# MinIO console at http://localhost:9001 (minioadmin/minioadmin)
# PostgreSQL at localhost:5433
```

**Option B: Run services separately** (faster iteration)

```bash
# Start only the dependencies
docker compose -f docker-compose.dev.yml up postgres minio -d

# Frontend dev server with hot reload (port 5173, proxies API to 8080)
make dev-web

# Go server (in another terminal)
make run
```

### Running Tests

```bash
make test                   # Go tests
cd web && pnpm typecheck    # Frontend type checking
```

### Building

```bash
make build    # Builds frontend + Go binary into bin/sendrec
```

## API Reference

Interactive API documentation is available at [`/api/docs`](https://app.sendrec.eu/api/docs). The OpenAPI 3.0 spec lives at `internal/docs/openapi.yaml` — update it when adding or changing endpoints.

## Submitting Changes

1. Fork the repository and create a branch from `main`.
2. Make your changes. Write tests for new functionality.
3. If you added or changed API endpoints, update `internal/docs/openapi.yaml`.
4. Run `make test` and `make build` to verify everything works.
5. Open a pull request against `main`.

Keep PRs focused on a single change. If you're fixing a bug and also refactoring nearby code, split them into separate PRs.

## Code Style

**Go:** Follow standard Go conventions. CI runs [golangci-lint](https://golangci-lint.run/) on every PR. Run it locally:

```bash
golangci-lint run
```

**TypeScript/React:** Follow the existing patterns in `web/src/`. CI runs `tsc --noEmit` for type checking.

## Reporting Bugs

Use the [bug report template](https://github.com/sendrec/sendrec/issues/new?template=bug_report.yml) to file issues. Include steps to reproduce and any relevant logs.

## Suggesting Features

Open a [feature request](https://github.com/sendrec/sendrec/issues/new?template=feature_request.yml). Describe the problem you're solving before proposing a solution.

## License

By contributing to SendRec, you agree that your contributions will be licensed under the [AGPLv3](LICENSE).
