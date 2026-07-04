#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DAO_DIR="${ROOT}/server/repository/dao"

if ! command -v mockery >/dev/null 2>&1; then
  echo "mockery not found; install with:" >&2
  echo "  go install github.com/vektra/mockery/v2@v2.53.5" >&2
  exit 1
fi

cd "${DAO_DIR}"
mockery --output=./mocks --outpkg=mocks --all

echo "DAO mocks generated under ${DAO_DIR}/mocks"
