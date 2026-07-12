#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_INSTALLER="$SCRIPT_DIR/../install.sh"

if [[ -f "$ROOT_INSTALLER" ]]; then
  exec "$ROOT_INSTALLER" "$@"
fi

BRANCH="${EREBRUS_BRANCH:-main}"
exec bash <(curl -fsSL "https://erebrus.io/install.sh") "$@"
