#!/bin/sh
set -eu

repo="dreuse/prdash"
bindir="${PRDASH_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
	echo "install: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

need curl
need tar

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
linux | darwin) ;;
*) fail "unsupported system $os, use install.ps1 on windows" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) fail "unsupported architecture $arch" ;;
esac

version="${PRDASH_VERSION:-}"
if [ -z "$version" ]; then
	latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest") ||
		fail "could not reach github"
	version=${latest##*/}
fi
case "$version" in
v*) ;;
*) fail "could not resolve a release tag, got '$version'" ;;
esac

name="prdash_${version}_${os}_${arch}"
base="https://github.com/$repo/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading prdash $version for $os/$arch"
curl -fsSL "$base/$name.tar.gz" -o "$tmp/$name.tar.gz" || fail "no release asset for $os/$arch at $version"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "could not download checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
	sum=$(sha256sum "$tmp/$name.tar.gz" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
	sum=$(shasum -a 256 "$tmp/$name.tar.gz" | cut -d' ' -f1)
else
	fail "need sha256sum or shasum to verify the download"
fi

published=$(awk -v want="$name.tar.gz" '{ sub(/^\*/, "", $2); if ($2 == want) print $1 }' "$tmp/checksums.txt")
[ -n "$published" ] || fail "$name.tar.gz is not listed in checksums.txt, refusing to install it"
[ "$sum" = "$published" ] || fail "$name.tar.gz does not match its published checksum, refusing to install it"

tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
mkdir -p "$bindir"
mv "$tmp/$name/prdash" "$bindir/prdash"
chmod 755 "$bindir/prdash"

echo "installed prdash $version to $bindir"

case ":$PATH:" in
*":$bindir:"*) ;;
*) echo "add $bindir to your PATH to run it" ;;
esac

command -v gh >/dev/null 2>&1 || echo "prdash needs the gh cli, see https://cli.github.com"
