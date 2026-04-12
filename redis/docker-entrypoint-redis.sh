#!/bin/sh
set -e

# Start Redis in the background so we can run FUNCTION LOAD against it
redis-server /usr/local/etc/redis/redis.conf &
REDIS_PID=$!

# Wait until Redis is accepting connections
until redis-cli ping 2>/dev/null | grep -q PONG; do
  echo "Waiting for Redis..."
  sleep 0.1
done

# Load (or replace) the Lua function library
redis-cli FUNCTION LOAD REPLACE "$(cat /scripts/redis_markov.lua)"
echo "Markov Lua library loaded."

# Hand off to the background Redis process
wait "$REDIS_PID"
