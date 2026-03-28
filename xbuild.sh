#!/usr/bin/env bash
set -euox pipefail

OS="linux"
ARCH="amd64"
REGISTRY=""
#REGISTRY="easzlab.io.local:5000"
IMAGE_TAG="v1.0.0"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

svc="depth2stl-server"
./build.sh "$OS" "$ARCH"

if [[ -n "$REGISTRY" ]]; then
  IMAGE_NAME="$REGISTRY/$svc:$IMAGE_TAG"
else
  IMAGE_NAME="$svc:$IMAGE_TAG"
fi
echo "building image: $IMAGE_NAME"

docker build \
  --platform "$OS/$ARCH" \
  --build-arg GIT_BRANCH="$(git rev-list -1 HEAD)" \
  --build-arg GIT_HASH="$(git branch --show-current)" \
  -t "$IMAGE_NAME" \
  -f Dockerfile \
  "$SCRIPT_DIR"

if [[ -n "$REGISTRY" ]]; then
  docker push "$IMAGE_NAME"
fi

docker save "$IMAGE_NAME" -o depth2stl-server.tar
scp depth2stl-server.tar ubuntu@49.234.207.56:/home/ubuntu
