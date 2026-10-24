#!/bin/sh
# Exercise the command surface in isolated state; no network or service changes.
set -eu
bin=$1
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
export HOME="$scratch/home" XDG_CONFIG_HOME="$scratch/config" AppData="$scratch/config"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"
"$bin" --help >/dev/null
"$bin" sub --help >/dev/null
"$bin" list >/dev/null
"$bin" sub list >/dev/null
"$bin" status >/dev/null
"$bin" status --json >/dev/null
"$bin" off --stop >/dev/null
"$bin" env --unset >/dev/null
printf '%s\n' 'CLI smoke passed (isolated state)'
