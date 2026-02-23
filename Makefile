.PHONY: run test build build-web dev-web docker-up docker-down e2e-up e2e-down e2e-test

# Run Go server (requires DATABASE_URL, S3 env vars)
run:
	go run ./cmd/sendrec

# Run all tests
test:
	go test ./...

# Build Go binary (requires web/dist to exist)
build: build-web
	go build -ldflags="-X main.version=$$(git describe --tags --always --dirty)" -o bin/sendrec ./cmd/sendrec

# Build frontend
build-web:
	cd web && pnpm install && pnpm build

# Frontend dev server (proxies API to localhost:8080)
dev-web:
	cd web && pnpm dev

# Start full stack via Docker Compose
docker-up:
	docker compose -f docker-compose.dev.yml up --build -d

# Stop full stack
docker-down:
	docker compose -f docker-compose.dev.yml down

# Start e2e test environment
e2e-up:
	docker compose -f docker-compose.e2e.yml up --build -d

# Stop e2e test environment
e2e-down:
	docker compose -f docker-compose.e2e.yml down -v

# Run e2e tests (requires e2e environment running)
e2e-test:
	cd web && pnpm e2e
