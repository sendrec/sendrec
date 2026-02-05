.PHONY: run test build build-web dev-web docker-up docker-down

# Run Go server (requires DATABASE_URL, S3 env vars)
run:
	go run ./cmd/sendrec

# Run all tests
test:
	go test ./...

# Build Go binary (requires web/dist to exist)
build: build-web
	go build -o bin/sendrec ./cmd/sendrec

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
