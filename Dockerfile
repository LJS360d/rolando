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
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make dave vosk
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
    ca-certificates \
    # curl \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Piper TTS
# RUN mkdir -p /usr/share/piper-voices && \
#     curl -fsSL https://github.com/rhasspy/piper/releases/latest/download/piper_amd64.tar.gz | \
#     tar -xz -C /usr/local/bin --strip-components=1 piper/piper && \
#     chmod +x /usr/local/bin/piper

# # Download Piper voice models for supported languages
# RUN mkdir -p /usr/share/piper-voices/en/en_US/lessac/medium && \
#     mkdir -p /usr/share/piper-voices/it/it_IT/riccardo/x_low && \
#     mkdir -p /usr/share/piper-voices/de/de_DE/thorsten/medium && \
#     mkdir -p /usr/share/piper-voices/es/es_ES/ald/medium && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx" -o "/usr/share/piper-voices/en/en_US/lessac/medium/en_US-lessac-medium.onnx" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json" -o "/usr/share/piper-voices/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx" -o "/usr/share/piper-voices/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx.json" -o "/usr/share/piper-voices/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx.json" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx" -o "/usr/share/piper-voices/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx.json" -o "/usr/share/piper-voices/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx.json" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/ald/medium/es_ES-ald-medium.onnx" -o "/usr/share/piper-voices/es/es_ES/ald/medium/es_ES-ald-medium.onnx" && \
#     curl -fsSL "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/ald/medium/es_ES-ald-medium.onnx.json" -o "/usr/share/piper-voices/es/es_ES/ald/medium/es_ES-ald-medium.onnx.json"

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
