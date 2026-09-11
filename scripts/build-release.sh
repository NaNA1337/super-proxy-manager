#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
version="${1:?Usage: scripts/build-release.sh 1.0.1}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Expected a numeric x.y.z version' >&2; exit 1; }
revision=$(git rev-parse HEAD)
[ -z "$(git status --porcelain --untracked-files=normal)" ] || { echo 'Commit source changes before building release artifacts' >&2; exit 1; }
output="$PWD/dist/v$version"
[ ! -e "$output" ] || { echo "Output already exists: $output" >&2; exit 1; }
# Verify the committed embedded frontend matches a clean frontend build.
npm --prefix frontend ci
npm --prefix frontend run build
diff -qr frontend/dist backend/embedded/dist
mkdir -p "$output"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for arch in amd64 arm64; do
    binary="$output/super-proxy-web-linux-$arch"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=$version -X main.commit=$revision" -o "$binary" ./backend/cmd/server
    package="super-proxy-manager-v${version}-linux-$arch"
    mkdir -p "$work/$package"
    cp "$binary" "$work/$package/super-proxy-web"
    cp README.md "$work/$package/"
    cp -r deploy docs "$work/$package/"
    tar -C "$work" -czf "$output/$package.tar.gz" "$package"
done
printf 'version=%s\ncommit=%s\n' "$version" "$revision" > "$output/build-info.txt"
go version >> "$output/build-info.txt"
(cd "$output" && sha256sum super-proxy-web-* *.tar.gz build-info.txt > checksums.txt)
printf 'Release artifacts: %s\n' "$output"
