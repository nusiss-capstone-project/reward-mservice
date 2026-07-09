#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMMON_MODULE="github.com/nusiss-capstone-project/reward-mservice/common"
COMMON_VERSION="${1:-${COMMON_VERSION:-}}"

if [[ -z "${COMMON_VERSION}" ]]; then
  echo "usage: $0 <common-version>" >&2
  echo "example: $0 v0.0.2" >&2
  echo "or: COMMON_VERSION=v0.0.2 $0" >&2
  exit 1
fi

MODULES=(client server)

for module in "${MODULES[@]}"; do
  echo "==> upgrading ${COMMON_MODULE}@${COMMON_VERSION} in ${module}"
  (
    cd "${ROOT}/${module}"
    go get "${COMMON_MODULE}@${COMMON_VERSION}"
    go mod tidy
  )
done

echo "common upgraded to ${COMMON_VERSION} in: ${MODULES[*]}"
