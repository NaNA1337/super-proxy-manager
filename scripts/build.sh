#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
npm --prefix frontend ci
npm --prefix frontend run build
rm -rf backend/embedded/dist
mkdir -p backend/embedded/dist
cp -r frontend/dist/. backend/embedded/dist/
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/super-proxy-web ./backend/cmd/server
