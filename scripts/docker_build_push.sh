#!/usr/bin/env bash
set -euo pipefail

# Build and optionally push Docker image for this project.
# Usage:
#   ./scripts/docker_build_push.sh [tag] [push]
# Examples:
#   ./scripts/docker_build_push.sh myrepo/api:1.0.0
#   ./scripts/docker_build_push.sh myrepo/api:1.0.0 push

TAG=${1:-kidpech/api_free_demo:latest}
DO_PUSH=${2:-}

echo "Building Docker image: ${TAG}"
docker build -t "${TAG}" -f Dockerfile .

if [[ "${DO_PUSH}" == "push" ]]; then
  echo "Pushing ${TAG}"
  docker push "${TAG}"
fi

echo "Done."
