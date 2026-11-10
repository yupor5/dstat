#!/usr/bin/env bash
# Refresh the APT repository (pool + dists/stable) inside a gh-pages checkout.
#
# Usage: packaging/mkrepo-deb.sh <repo-root> <packages-dir>
#   repo-root     directory that contains the published site (public/)
#   packages-dir  directory with the freshly built .deb files
#
# Layout produced:
#   deb/pool/*.deb
#   deb/dists/stable/main/binary-<arch>/Packages(.gz)
#   deb/dists/stable/Release
#
# The repository is unsigned on purpose (no GPG private key in CI), so clients
# must use `deb [trusted=yes] ...`.  See README for signed-repo instructions.
set -euo pipefail

root="$1"
pkgs="$2"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for tool in dpkg-scanpackages apt-ftparchive gzip; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "$tool not found (apt-get install dpkg-dev apt-utils gzip)" >&2
		exit 1
	}
done

# dpkg-scanpackages must see the pool through a path relative to the repository
# root, otherwise it writes absolute Filename entries into Packages and apt
# cannot resolve them.
debroot="$root/deb"
mkdir -p "$debroot/pool" "$debroot/dists/stable/main"
cp -f "$pkgs"/*.deb "$debroot/pool/"

cd "$debroot"

for arch in amd64 arm64 i386 armhf; do
	dir="dists/stable/main/binary-$arch"
	mkdir -p "$dir"
	echo "==> Packages for $arch"
	dpkg-scanpackages --arch "$arch" pool /dev/null 2>/dev/null > "$dir/Packages"
	gzip -9c "$dir/Packages" > "$dir/Packages.gz"
done

echo "==> Release"
apt-ftparchive -c "$script_dir/apt-release.conf" release dists/stable > dists/stable/Release
sed -n '1,12p' dists/stable/Release
