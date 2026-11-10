#!/usr/bin/env bash
# Refresh the RPM repository (repodata) inside a gh-pages checkout.
#
# Usage: packaging/mkrepo-rpm.sh <repo-root> <packages-dir>
#   repo-root     directory that contains the published site (public/)
#   packages-dir  directory with the freshly built .rpm files
#
# Old package versions are kept: they are copied into <repo-root>/rpm/ and
# createrepo_c --update only refreshes the metadata.
set -euo pipefail

root="$1"
pkgs="$2"

if ! command -v createrepo_c >/dev/null 2>&1; then
	echo "createrepo_c not found (dnf install createrepo_c / apt-get install createrepo-c)" >&2
	exit 1
fi

mkdir -p "$root/rpm"
cp -f "$pkgs"/*.rpm "$root/rpm/"

echo "==> createrepo_c $root/rpm"
createrepo_c --update --quiet "$root/rpm"
ls -1 "$root/rpm"/*.rpm
