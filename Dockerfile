FROM golang:1.23-bullseye AS builder

# build dependencies, including libopus-dev and libopusfile-dev
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libopus-dev \
    libopusfile-dev \
    pkg-config \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make build

FROM debian:bullseye-slim

WORKDIR /root/

# runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    sqlite3 \
    libsqlite3-0 \
    libopusfile0 \
    ffmpeg \
    espeak-ng \
    ca-certificates \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/bin/main .
COPY --from=builder /app/.env .
COPY --from=builder /app/vosk /root/vosk
ENV LD_LIBRARY_PATH="/root/vosk/lib:$LD_LIBRARY_PATH"
ENV GO_ENV="production"

EXPOSE 8080

CMD ["./main"]