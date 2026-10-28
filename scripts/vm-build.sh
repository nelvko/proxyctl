#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
go_version=$(sed -n 's/^go //p' "$project_dir/go.mod" | head -n 1)
case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported VM architecture: $(uname -m)" >&2; exit 2 ;;
esac
output_file=${1:-"$project_dir/dist/proxyctl-linux-${arch}"}
case "$output_file" in
  /*) ;;
  *) output_file="$project_dir/$output_file" ;;
esac

go_bin="$HOME/.local/go/bin/go"
if [[ ! -x "$go_bin" ]]; then
  mkdir -p "$HOME/.local"
  install_dir=$(mktemp -d "$HOME/.local/.go-install.XXXXXX")
  trap 'rm -rf "$install_dir"' EXIT
  curl -fsSL "https://go.dev/dl/go${go_version}.linux-${arch}.tar.gz" | tar -xz -C "$install_dir"
  mv "$install_dir/go" "$HOME/.local/go"
  rmdir "$install_dir"
  trap - EXIT
fi

build_dir=$(mktemp -d /tmp/proxyctl-build.XXXXXX)
trap 'rm -rf "$build_dir"' EXIT
tar -c -C "$project_dir" --exclude .git --exclude dist . | tar -x -C "$build_dir"
GOWORK=off "$go_bin" build -C "$build_dir" -o "$build_dir/proxyctl-linux-${arch}" .
mkdir -p "$(dirname "$output_file")"
cp "$build_dir/proxyctl-linux-${arch}" "$output_file"
echo ">> built $output_file"
