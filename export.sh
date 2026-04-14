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
# Layer cache is enabled by default. If you hit a weird cache bug, pass
# --no-cache as a one-off: `docker build --no-cache -t ... .`
docker build \
  -t "${IMAGE_NAME}:${IMAGE_TAG}" \
  "${SCRIPT_DIR}"

echo "==> Exporting image..."
docker save "${IMAGE_NAME}:${IMAGE_TAG}" | gzip > "${SCRIPT_DIR}/teenet-wallet.tar.gz"

echo "Done: ${SCRIPT_DIR}/teenet-wallet.tar.gz"
