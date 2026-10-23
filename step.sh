#!/usr/bin/env bash
# Compatibility entrypoint for callers that run step.sh outside Bitrise.
# Bitrise builds main.go directly through the Go toolkit.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d "${TMPDIR:-/tmp}/firebase-app-distribution.XXXXXX")"
trap 'rm -rf "$build_dir"' EXIT

(cd "$script_dir" && go build -mod=vendor -o "$build_dir/firebase-app-distribution" .)
"$build_dir/firebase-app-distribution"
