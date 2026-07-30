# Local / CI image build for the QR-login fork
#
# Usage (from repo root, Docker Desktop running):
#   ./deploy/truenas/build-and-push.sh
# Or build only:
#   ./deploy/truenas/build-and-push.sh --no-push

set -euo pipefail

IMAGE="${IMAGE:-ghcr.io/dylanl321/homebox}"
TAG="${TAG:-main}"
NO_PUSH=0

for arg in "$@"; do
  case "$arg" in
    --no-push) NO_PUSH=1 ;;
  esac
done

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo head)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
VERSION="${VERSION:-fork-qr-${COMMIT}}"

echo "Building ${IMAGE}:${TAG} (commit=${COMMIT})"
docker build \
  -f Dockerfile \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  --build-arg VERSION="${VERSION}" \
  -t "${IMAGE}:${TAG}" \
  .

if [[ "${NO_PUSH}" -eq 1 ]]; then
  echo "Skipping push (--no-push). Image ready: ${IMAGE}:${TAG}"
  exit 0
fi

echo "Pushing ${IMAGE}:${TAG}"
docker push "${IMAGE}:${TAG}"
echo "Done. TrueNAS can pull ${IMAGE}:${TAG}"
