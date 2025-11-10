#!/usr/bin/env bash
set -euo pipefail

COUNT=${COUNT:-3}
BASE_PORT=${BASE_PORT:-8080}
DIR_PORT=${DIR_PORT:-8081}
# Normalize if passed like count=3 base_port=8080 dir_port=8081 (from Just)
COUNT=${COUNT#count=}; COUNT=${COUNT//\"/}
BASE_PORT=${BASE_PORT#base_port=}; BASE_PORT=${BASE_PORT//\"/}
DIR_PORT=${DIR_PORT#dir_port=}; DIR_PORT=${DIR_PORT//\"/}
# Extract digits only to be safe (handles stray prefixes)
COUNT=$(printf "%s" "$COUNT" | sed -E 's/[^0-9]*([0-9]+).*/\1/')
BASE_PORT=$(printf "%s" "$BASE_PORT" | sed -E 's/[^0-9]*([0-9]+).*/\1/')
DIR_PORT=$(printf "%s" "$DIR_PORT" | sed -E 's/[^0-9]*([0-9]+).*/\1/')
# Defaults if empty
[ -z "$COUNT" ] && COUNT=3
[ -z "$BASE_PORT" ] && BASE_PORT=8080
[ -z "$DIR_PORT" ] && DIR_PORT=8081
echo "Config → COUNT=$COUNT BASE_PORT=$BASE_PORT DIR_PORT=$DIR_PORT"
JWT_SECRET=${JWT_SECRET:-dev_jwt_secret}
REFRESH_JWT_SECRET=${REFRESH_JWT_SECRET:-dev_refresh_secret}
CORS_ORIGINS=${CORS_ORIGINS:-http://localhost:5173}
NETWORK=${NETWORK:-smalltalk-net}

echo "Building images..."
docker build -f tests/load/docker/Dockerfile.app -t smalltalk-app:local .
docker build -f tests/load/docker/Dockerfile.directory -t smalltalk-directory:local .

echo "Ensuring network ${NETWORK}..."
docker network inspect "${NETWORK}" >/dev/null 2>&1 || docker network create "${NETWORK}"

echo "Starting redis..."
docker rm -f smalltalk-redis >/dev/null 2>&1 || true
docker run -d --name smalltalk-redis --network "${NETWORK}" redis:7-alpine

echo "Starting directory on port ${DIR_PORT}..."
docker rm -f smalltalk-directory >/dev/null 2>&1 || true
docker run -d \
  --name smalltalk-directory \
  --network "${NETWORK}" \
  -p "${DIR_PORT}:8081" \
  -e DIRECTORY_PORT=8081 \
  -e DIRECTORY_JWT_SECRET="${JWT_SECRET}" \
  -e DIRECTORY_CORS_ORIGINS="${CORS_ORIGINS}" \
  -e REDIS_ADDR="smalltalk-redis:6379" \
  smalltalk-directory:local

echo "Starting ${COUNT} app instances..."
for i in $(seq 1 "${COUNT}"); do
  host_port=$((BASE_PORT + i - 1))
  name="smalltalk-app-${i}"
  docker rm -f "${name}" >/dev/null 2>&1 || true
  docker run -d \
    --name "${name}" \
    --network "${NETWORK}" \
    -p "${host_port}:8080" \
    -e PORT=8080 \
    -e CORS_ORIGINS="${CORS_ORIGINS}" \
    -e JWT_SECRET="${JWT_SECRET}" \
    -e REFRESH_JWT_SECRET="${REFRESH_JWT_SECRET}" \
    -e DIRECTORY_URL="http://smalltalk-directory:8081" \
    -e WS_PUBLIC_URL="ws://localhost:${host_port}/ws" \
    -e REDIS_ADDR="smalltalk-redis:6379" \
    smalltalk-app:local
  echo "  - ${name} on ws://localhost:${host_port}/ws"
done

echo
echo "Cluster up."
echo "Directory:  http://localhost:${DIR_PORT}"
echo "Apps:       ws://localhost:${BASE_PORT}.. ws range (${COUNT} instances)"
echo "Run k6 with: DIR=http://localhost:${DIR_PORT} ROOM=gaming TOKEN=<jwt> k6 run tests/load/join-and-ws.js"


