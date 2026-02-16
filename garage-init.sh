#!/bin/sh
set -e

S3_ACCESS_KEY="${S3_ACCESS_KEY:-sendrec-dev-key}"
S3_SECRET_KEY="${S3_SECRET_KEY:-sendrec-dev-secret}"
S3_BUCKET="${S3_BUCKET:-recordings}"

echo "Waiting for Garage to be ready..."
until garage status > /dev/null 2>&1; do
  sleep 1
done
echo "Garage is ready."

NODE_ID=$(garage status 2>/dev/null | grep -oE '[a-f0-9]{16}' | head -1)
echo "Node ID: ${NODE_ID}"

echo "Assigning layout..."
garage layout assign -z dc1 -c 1G "${NODE_ID}" 2>/dev/null || true

echo "Applying layout..."
garage layout apply --version 1 2>/dev/null || true

echo "Importing API key..."
garage key import --yes "${S3_ACCESS_KEY}" "${S3_SECRET_KEY}" 2>/dev/null || true

echo "Creating bucket '${S3_BUCKET}'..."
garage bucket create "${S3_BUCKET}" 2>/dev/null || true

echo "Granting key access to bucket..."
garage bucket allow --read --write --owner "${S3_BUCKET}" --key "${S3_ACCESS_KEY}" 2>/dev/null || true

echo "Garage initialization complete."
