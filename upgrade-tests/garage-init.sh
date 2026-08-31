#!/bin/sh
set -eu

keys_file=/run/garage-keys/env

until garage status >/dev/null 2>&1; do
  sleep 1
done

node_id=$(garage status 2>/dev/null | grep -oE '[a-f0-9]{16}' | head -1)
garage layout assign -z upgradeproof -c 1G "$node_id" >/dev/null 2>&1 || true
garage layout apply --version 1 >/dev/null 2>&1 || true
if [ -f "$keys_file" ]; then
  . "$keys_file"
else
  key_info=$(garage key create upgradeproof)
  S3_ACCESS_KEY=$(printf '%s\n' "$key_info" | grep -oE 'GK[a-f0-9]{24}' | head -1)
  S3_SECRET_KEY=$(printf '%s\n' "$key_info" | grep 'Secret key' | sed 's/.*: *//')
  test -n "$S3_ACCESS_KEY"
  test -n "$S3_SECRET_KEY"
  umask 077
  printf 'S3_ACCESS_KEY=%s\nS3_SECRET_KEY=%s\n' "$S3_ACCESS_KEY" "$S3_SECRET_KEY" > "$keys_file"
fi
chmod 0444 "$keys_file"
garage bucket create recordings >/dev/null 2>&1 || true
garage bucket allow --read --write --owner recordings --key "$S3_ACCESS_KEY" >/dev/null

echo "Garage upgrade-test bucket is ready."
