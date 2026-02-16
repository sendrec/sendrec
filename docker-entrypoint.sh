#!/bin/sh
# Source Garage-generated S3 credentials if available
GARAGE_KEYS_FILE="${GARAGE_KEYS_FILE:-/run/garage-keys/env}"
if [ -f "${GARAGE_KEYS_FILE}" ]; then
  set -a
  . "${GARAGE_KEYS_FILE}"
  set +a
fi
exec "$@"
