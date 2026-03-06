# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.26.1-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o sendrec ./cmd/sendrec

# Stage 3: Final image (base includes whisper-cli, ffmpeg, RNNoise model)
FROM alexneamtu/sendrec-base:latest
COPY --from=backend /app/sendrec .
COPY docker-entrypoint.sh .
USER sendrec
EXPOSE 8080
ENTRYPOINT ["./docker-entrypoint.sh"]
CMD ["./sendrec"]
