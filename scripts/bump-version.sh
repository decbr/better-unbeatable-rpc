#!/usr/bin/env bash
set -euo pipefail

usage() { echo "Usage: scripts/bump-version.sh X.Y.Z" >&2; exit 1; }
die() { echo "bump-version: $*" >&2; exit 1; }

[[ $# -eq 1 && "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || usage
version="$1"

cd "$(dirname "${BASH_SOURCE[0]}")/.."
files=(Makefile plugin/UnbeatableRPC.csproj plugin/src/Plugin.cs plugin/dist/UnbeatableRPC.dll)

git diff --quiet HEAD -- Makefile plugin || die "commit or stash your changes to Makefile and plugin/ first"
git rev-parse -q --verify "refs/tags/v$version" >/dev/null && die "tag v$version already exists"

sed -i "s/^VERSION  ?= .*/VERSION  ?= $version/" Makefile
sed -i "s#<Version>.*</Version>#<Version>$version</Version>#" plugin/UnbeatableRPC.csproj
sed -i "s/public const string Version = \".*\";/public const string Version = \"$version\";/" plugin/src/Plugin.cs

make plugin-dist VERSION="$version"

git add -- "${files[@]}"
git commit -q -m "chore: bump version to $version" -- "${files[@]}"
git tag -a "v$version" -m "v$version"

echo "Tagged v$version. Publish it with:"
echo "    git push --follow-tags"
