#!/usr/bin/env bash
set -euo pipefail

COUNT=${COUNT:-3}
BASE_PORT=${BASE_PORT:-8080}
DIR_PORT=${DIR_PORT:-8081}

docker rm -f smalltalk-directory >/dev/null 2>&1 || true
docker rm -f smalltalk-redis >/dev/null 2>&1 || true

for i in $(seq 1 "${COUNT}"); do
  docker rm -f "smalltalk-app-${i}" >/dev/null 2>&1 || true
done

echo "Cluster stopped."


