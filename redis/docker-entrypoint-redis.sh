#!/bin/sh
set -e

# Start Redis in the background so we can run FUNCTION LOAD against it.
# The config uses ${REDIS_PASSWORD} substitution — pass it via env so
# redis-server can expand it at runtime.
redis-server /usr/local/etc/redis/redis.conf --requirepass "$REDIS_PASSWORD" &
REDIS_PID=$!

# Wait until Redis is accepting connections (auth required now).
until redis-cli -a "$REDIS_PASSWORD" ping 2>/dev/null | grep -q PONG; do
  echo "Waiting for Redis..."
  sleep 0.1
done

# Load (or replace) the Lua function library.
redis-cli -a "$REDIS_PASSWORD" FUNCTION LOAD REPLACE "$(cat /scripts/redis_markov.lua)"
echo "Markov Lua library loaded."

# Hand off to the background Redis process.
wait "$REDIS_PID"
