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
COPY --from=builder --chown=appuser:appgroup /app/dave ./dave
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh
ENV LD_LIBRARY_PATH="/home/appuser/vosk/lib:/home/appuser/dave/lib:$LD_LIBRARY_PATH"
ENV GO_ENV="production"
ENV PATH="/home/appuser:$PATH"

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
