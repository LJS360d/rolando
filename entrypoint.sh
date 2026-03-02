#!/bin/sh
set -e
if [ -d /home/appuser/data ]; then
	chown -R appuser:appgroup /home/appuser/data
fi
exec su appuser -c "exec ./main"
