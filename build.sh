#!/usr/bin/env bash
set -euo pipefail

goos="${1:-}"
case "$goos" in
linux | darwin | "") ;;
*) echo "illegal os: $goos" && exit 1 ;;
esac

goarch="${2:-}"
case "$goarch" in
amd64 | arm64 | "") ;;
*) echo "illegal arch: $goarch" && exit 1 ;;
esac

# 构建信息
git_hash=$(git rev-parse --short HEAD)
git_branch=$(git rev-parse --abbrev-ref HEAD)
build_time=$(date "+%Y-%m-%d %H:%M:%S %Z")

set -x
env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w -X main.gitHash=$git_hash -X main.gitBranch=$git_branch -X 'main.buildTime=$build_time'"
