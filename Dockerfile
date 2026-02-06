# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o sendrec ./cmd/sendrec

# Stage 3: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg
RUN addgroup -S sendrec && adduser -S sendrec -G sendrec
WORKDIR /app
COPY --from=backend /app/sendrec .
USER sendrec
EXPOSE 8080
CMD ["./sendrec"]
