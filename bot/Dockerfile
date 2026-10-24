# syntax=docker/dockerfile:1
FROM debian:13-slim AS assets
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    make \
    unzip

WORKDIR /app
COPY Makefile ./
RUN make dave vosk && find dave vosk -exec touch -h -d @0 {} +

FROM golang:1.26-trixie AS builder
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libopus-dev \
    libopusfile-dev \
    pkg-config

WORKDIR /app
COPY --from=assets /app/dave ./dave
COPY --from=assets /app/vosk/lib ./vosk/lib
RUN mkdir -p vosk/models
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN make build VERSION=$VERSION

FROM debian:13-slim
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
RUN useradd -u 1000 -m -d /app rolando && mkdir /data && chown rolando /data

WORKDIR /app
COPY --link --from=assets /app/vosk/models /app/vosk/models
COPY --link --from=assets /app/vosk/lib /app/vosk/lib
COPY --link --from=assets /app/dave/lib /app/dave/lib
COPY --link --from=builder /app/bin/rolando /usr/local/bin/rolando

ENV LD_LIBRARY_PATH=/app/vosk/lib:/app/dave/lib \
    GO_ENV=production \
    DATA_DIR=/data \
    SERVER_ADDRESS=0.0.0.0:8080
USER 1000
VOLUME /data
EXPOSE 8080 9000
ENTRYPOINT ["rolando"]
