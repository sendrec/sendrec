#!/bin/sh
set -e

S3_BUCKET="${S3_BUCKET:-recordings}"
GARAGE_KEYS_FILE="${GARAGE_KEYS_FILE:-/run/garage-keys/env}"

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

# Create or reuse API key
echo "Creating API key..."
KEY_INFO=$(garage key create sendrec-key 2>/dev/null || true)

# If key already exists, list and find it
if [ -z "${KEY_INFO}" ]; then
  KEY_INFO=$(garage key info sendrec-key 2>/dev/null || true)
fi

# Extract key ID from output (GK... format)
KEY_ID=$(echo "${KEY_INFO}" | grep -oE 'GK[a-f0-9]{24}' | head -1)
SECRET=$(echo "${KEY_INFO}" | grep "Secret key" | sed 's/.*: *//')

if [ -n "${KEY_ID}" ] && [ -n "${SECRET}" ]; then
  echo "Key ID: ${KEY_ID}"
  # Write credentials for sendrec to read
  mkdir -p "$(dirname "${GARAGE_KEYS_FILE}")"
  printf 'S3_ACCESS_KEY=%s\nS3_SECRET_KEY=%s\n' "${KEY_ID}" "${SECRET}" > "${GARAGE_KEYS_FILE}"
else
  echo "ERROR: Could not extract key credentials"
  exit 1
fi

echo "Creating bucket '${S3_BUCKET}'..."
garage bucket create "${S3_BUCKET}" 2>/dev/null || true

echo "Granting key access to bucket..."
if [ -n "${KEY_ID}" ]; then
  garage bucket allow --read --write --owner "${S3_BUCKET}" --key "${KEY_ID}" 2>/dev/null || true
fi

echo "Configuring CORS..."
BUCKET_ID=$(garage json-api GetBucketInfo "{\"globalAlias\":\"${S3_BUCKET}\"}" 2>/dev/null | grep '"id"' | head -1 | sed 's/.*: *"\([^"]*\)".*/\1/')
if [ -n "${BUCKET_ID}" ]; then
  garage json-api UpdateBucket "{\"id\":\"${BUCKET_ID}\",\"body\":{\"corsConfig\":{\"set\":[{\"allowedOrigins\":[\"*\"],\"allowedMethods\":[\"GET\",\"PUT\",\"HEAD\"],\"allowedHeaders\":[\"*\"],\"exposeHeaders\":[\"ETag\"],\"maxAgeSeconds\":3600}]}}}" > /dev/null 2>&1 || true
  echo "CORS configured."
else
  echo "WARNING: Could not configure CORS (bucket ID not found)"
fi

echo "Garage initialization complete."
