#!/usr/bin/env bash
# Removes the demo database container and volume (make demo-down).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
compose down -v
