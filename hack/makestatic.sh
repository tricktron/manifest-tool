#!/usr/bin/env bash
set -Eeuo pipefail

BINARY='manifest-tool'

COMMIT="$(git rev-parse HEAD 2>/dev/null)"
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  COMMIT="${COMMIT}-dirty"
fi

LDFLAGS="-w -extldflags -static -X main.gitCommit=${COMMIT}"

FAILURES=()

cd v2
GOOS=linux
GOARCH="${1}"
GOARM="${2-''}"

ARCH_ENV="GOOS=${GOOS} GOARCH=${GOARCH}"
if [ "${GOARCH}" = 'arm' ]; then
  [ -z "${GOARM}" ] || echo >&2 "WARNING: missing GOARM value for $GOARCH in ${BASH_SOURCE[0]}"
  ARCH_ENV="${ARCH_ENV} GOARM=${GOARM}"
fi

CMD="${ARCH_ENV} CGO_ENABLED=0 GO_EXTLINK_ENABLED=0 go build -ldflags \"${LDFLAGS}\" -o ../manifest-tool -tags netgo -installsuffix netgo github.com/estesp/manifest-tool/v2/cmd/manifest-tool"
echo "${CMD}"
eval "${CMD}" || FAILURES=( "${FAILURES[@]}" "${GOOS}/${GOARCH}" )
cd ..

# eval errors
if [ "${#FAILURES[@]}" -gt 0 ]; then
  echo >&2
  echo >&2 "ERROR: ${BINARY} build failed on: ${FAILURES[*]}"
  echo >&2
  exit 1
fi
