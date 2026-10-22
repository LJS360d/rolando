# syntax=docker/dockerfile:1
FROM golang:1.26-trixie AS builder
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    unzip \
    libopus-dev \
    libopusfile-dev \
    pkg-config

WORKDIR /app
COPY Makefile ./
RUN make dave vosk
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN make build VERSION=$VERSION

FROM debian:13-slim
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN useradd -u 1000 -m -d /app rolando && mkdir /data && chown rolando /data

WORKDIR /app
COPY --from=builder /app/vosk ./vosk
COPY --from=builder /app/dave/lib ./dave/lib
COPY --from=builder /app/bin/rolando /usr/local/bin/rolando

ENV LD_LIBRARY_PATH=/app/vosk/lib:/app/dave/lib \
    GO_ENV=production \
    DATA_DIR=/data \
    SERVER_ADDRESS=0.0.0.0:8080
USER rolando
VOLUME /data
EXPOSE 8080 9000
ENTRYPOINT ["rolando"]
