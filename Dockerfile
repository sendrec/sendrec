# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Stage 2: Build whisper.cpp
FROM alpine:3.21 AS whisper
RUN apk add --no-cache build-base cmake git
WORKDIR /build
RUN git clone --depth 1 --branch v1.8.3 https://github.com/ggerganov/whisper.cpp.git . && \
    cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF && \
    cmake --build build --target whisper-cli -j$(nproc)

# Stage 3: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o sendrec ./cmd/sendrec

# Stage 4: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg libstdc++
RUN addgroup -S sendrec && adduser -S sendrec -G sendrec
WORKDIR /app
COPY --from=backend /app/sendrec .
COPY --from=whisper /build/build/bin/whisper-cli /usr/local/bin/whisper-cli
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
USER sendrec
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./sendrec"]
