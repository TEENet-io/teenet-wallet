#!/usr/bin/env bash
# Build the teenet-wallet Docker image (frontend + backend) and export to user-management-system/static/.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-teenet-wallet}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "==> Updating submodules to latest..."
cd "${SCRIPT_DIR}"
git submodule update --init --remote --recursive

echo "==> Building Docker image ${IMAGE_NAME}:${IMAGE_TAG}..."
docker build --no-cache \
  -t "${IMAGE_NAME}:${IMAGE_TAG}" \
  "${SCRIPT_DIR}"

echo "==> Exporting image..."
docker save "${IMAGE_NAME}:${IMAGE_TAG}" | gzip > "${SCRIPT_DIR}/teenet-wallet.tar.gz"

echo "Done: ${SCRIPT_DIR}/teenet-wallet.tar.gz"
