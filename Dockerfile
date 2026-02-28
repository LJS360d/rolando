# syntax=docker/dockerfile:1
FROM golang:1.26-trixie AS builder
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libopus-dev \
    libopusfile-dev \
    pkg-config
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM debian:13-slim AS runner
RUN groupadd -r appgroup && useradd -r -g appgroup -m -d /home/appuser appuser
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    sqlite3 \
    libsqlite3-0 \
    libopusfile0 \
    ffmpeg \
    espeak-ng \
    ca-certificates \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /home/appuser
COPY --from=builder --chown=appuser:appgroup /app/bin/main .
COPY --from=builder --chown=appuser:appgroup /app/.env .
COPY --from=builder --chown=appuser:appgroup /app/vosk ./vosk
ENV LD_LIBRARY_PATH="/home/appuser/vosk/lib:$LD_LIBRARY_PATH" \
    GO_ENV="production" \
    PATH="/home/appuser:$PATH"

USER appuser
EXPOSE 8080
ENTRYPOINT ["./main"]
