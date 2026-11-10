#!/usr/bin/env bash
# Build the linux binary for one architecture and package it as .deb and .rpm.
#
# Usage: packaging/build-packages.sh <goarch> [goarm] <version> [outdir]
#   goarch   amd64 | arm64 | 386 | arm
#   goarm    GOARM value, only used for goarch=arm (7 for armv7)
#   version  package version without leading "v", e.g. 1.0.0
#   outdir   output directory, default: dist/packages
#
# Run from the repository root.  Requires go, nfpm and bash.
set -euo pipefail

arch="$1"
goarm="${2:-}"
version="$3"
outdir="${4:-dist/packages}"

case "$arch" in
	amd64) deb_arch=amd64; rpm_arch=x86_64 ;;
	arm64) deb_arch=arm64; rpm_arch=aarch64 ;;
	386)   deb_arch=i386;  rpm_arch=i686 ;;
	arm)   deb_arch=armhf; rpm_arch=armv7hl ;;
	*) echo "unsupported goarch: $arch" >&2; exit 1 ;;
esac

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

bindir="dist/bin/linux-$arch"
mkdir -p "$bindir" "$outdir" dist/stage
bin="$bindir/dstat-go"

echo "==> building linux/$arch (GOARM=${goarm:-n/a})"
if [ -n "$goarm" ]; then
	CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOARM="$goarm" \
		go build -trimpath -ldflags "-s -w" -o "$bin" .
else
	CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
		go build -trimpath -ldflags "-s -w" -o "$bin" .
fi

echo "==> packaging deb ($deb_arch)"
cp "$bin" dist/stage/dstat-go
PKG_ARCH="$deb_arch" PKG_VERSION="$version" \
	nfpm package --config packaging/nfpm.yaml --packager deb \
	--target "$outdir/dstat-go_${version}-1_${deb_arch}.deb"

echo "==> packaging rpm ($rpm_arch)"
cp "$bin" dist/stage/dstat-go
PKG_ARCH="$rpm_arch" PKG_VERSION="$version" \
	nfpm package --config packaging/nfpm.yaml --packager rpm \
	--target "$outdir/dstat-go-${version}-1.${rpm_arch}.rpm"

ls -l "$outdir"
