#!/usr/bin/env bash
set -euo pipefail

# ถ้าอยากเปลี่ยน registry ให้ตั้ง ENV ก่อนรัน เช่น
#   REGISTRY=ghcr.io/myorg ./scripts/deploy_public.sh
REGISTRY=${REGISTRY:-kidpechcode}
IMAGE=${IMAGE:-${REGISTRY}/api_free_demo}
TAG=${TAG:-latest}
NETWORK=${NETWORK:-demo-net}
POSTGRES_NAME=${POSTGRES_NAME:-postgres}
API_NAME=${API_NAME:-api}
DB_DSN=${DB_DSN:-postgres://postgres:postgres@${POSTGRES_NAME}:5432/demo_db?sslmode=disable}
REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
MIGRATIONS_PATH=${MIGRATIONS_PATH:-${REPO_ROOT}/migrations}

echo "[1/6] Cleaning previous containers"
docker rm -f "${POSTGRES_NAME}" "${API_NAME}" >/dev/null 2>&1 || true

echo "[2/6] Building local binary and image"
make docker-build

echo "[3/6] Tagging and pushing image to ${IMAGE}:${TAG}"
docker tag kidpech/api_free_demo:latest "${IMAGE}:${TAG}"
docker push "${IMAGE}:${TAG}"

echo "[4/6] Preparing Docker network ${NETWORK}"
docker network create "${NETWORK}" >/dev/null 2>&1 || true

echo "[5/6] Launching supporting containers"
docker run -d --name "${POSTGRES_NAME}" --network "${NETWORK}" \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_USER=postgres -e POSTGRES_DB=demo_db postgres:15

echo "Waiting for Postgres to accept connections..."
until docker exec "${POSTGRES_NAME}" pg_isready -U postgres >/dev/null 2>&1; do
  sleep 1
done

echo "[6/8] Applying database migrations"
docker run --rm --network "${NETWORK}" -v "${MIGRATIONS_PATH}:/migrations" \
  migrate/migrate -path=/migrations/postgres -database "${DB_DSN}" up

docker run -d --name "${API_NAME}" --network "${NETWORK}" -p 8080:8080 \
  --env-file .env "${IMAGE}:${TAG}"

echo "[7/8] Cloudflare tunnel note"
echo "NOTE: ตรวจสอบ ~/.cloudflared/config.yml ว่าชี้ไปที่ tunnel ที่สร้างไว้แล้ว"
echo "[8/8] Running Cloudflare Tunnel (press Ctrl+C to stop)"
cloudflared tunnel run api-demo