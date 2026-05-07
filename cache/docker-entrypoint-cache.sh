#!/bin/sh
set -e

valkey-server /usr/local/etc/cache/cache.conf --requirepass "$CACHE_PASSWORD" &
CACHE_PID=$!

until valkey-cli -a "$CACHE_PASSWORD" ping >/dev/null 2>&1; do
  echo "Waiting for cache service..."
  sleep 0.1
done

valkey-cli -a "$CACHE_PASSWORD" FUNCTION LOAD REPLACE "$(cat /scripts/cache_markov.lua)"
echo "Markov Lua library loaded."

wait "$CACHE_PID"
