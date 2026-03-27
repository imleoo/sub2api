#!/usr/bin/env bash
# =============================================================================
# Build and Push Docker Image to Docker Hub (ioke/myrepo)
# =============================================================================

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 0. Check if Docker daemon is running
if ! docker info >/dev/null 2>&1; then
    print_error "Docker daemon is not running. Please start Docker Desktop first."
    exit 1
fi

REPOSITORY="ioke/myrepo"
TAG="${1:-latest}"
IMAGE_FULL_NAME="${REPOSITORY}:${TAG}"

print_info "Preparing to build and push ${IMAGE_FULL_NAME}..."

# 1. Check if Buildx is available (required for multi-arch)
if ! docker buildx version >/dev/null 2>&1; then
    print_error "docker buildx is not installed. Please install it to support multi-arch builds."
    exit 1
fi

# 2. Check login status (optional, but helpful)
print_info "Checking Docker Hub login status..."
if ! docker system info | grep -q "Username: "; then
    print_info "Not logged in. Please run 'docker login' first."
    # We don't exit here as the push will fail later with a better error message if not logged in
fi

# 3. Create or use a buildx builder
BUILDER_NAME="sub2api-builder"
if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
    print_info "Creating new buildx builder: ${BUILDER_NAME}"
    docker buildx create --name "${BUILDER_NAME}" --use
else
    print_info "Using existing buildx builder: ${BUILDER_NAME}"
    docker buildx use "${BUILDER_NAME}"
fi

# 4. Build and Push
print_info "Building and pushing for linux/amd64 and linux/arm64..."
docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --tag "${IMAGE_FULL_NAME}" \
    --push \
    .

print_success "Successfully pushed ${IMAGE_FULL_NAME} to Docker Hub."
print_info "Image URL: https://hub.docker.com/r/${REPOSITORY}"
