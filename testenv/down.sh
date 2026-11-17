#!/usr/bin/env bash
# Removes the containers, network and env file of one profile.
set -euo pipefail
cd "$(dirname "$0")"
profile=${1:?usage: down.sh <profile>}
engine=${CONTAINER_ENGINE:-$(command -v podman >/dev/null 2>&1 && echo podman || echo docker)}
name="nbp-${profile//./-}"
"$engine" rm -f "$name" "${name}-redis" "${name}-pg" >/dev/null 2>&1 || true
"$engine" network rm "$name" >/dev/null 2>&1 || true
rm -f ".env.${profile}"
