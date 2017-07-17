#!/bin/bash

BINARY="manifest-tool"

GIT_BRANCH=`git rev-parse --abbrev-ref HEAD 2>/dev/null`
COMMIT=`git rev-parse HEAD 2>/dev/null`
[[ -n `git status --porcelain --untracked-files=no` ]] && {
  COMMIT="${COMMIT}-dirty"; }

LDFLAGS="-w -extldflags -static -X main.gitCommit=${COMMIT}"
LDFLAGS_OTHER="-X main.gitCommit=${COMMIT}"

# List of platforms we build binaries for at this time:
PLATFORMS="darwin/amd64 windows/amd64 linux/amd64" # OSX, Windows, Linux x86_64
PLATFORMS="$PLATFORMS linux/ppc64le linux/s390x"   # IBM POWER and z Systems
PLATFORMS="$PLATFORMS linux/arm linux/arm64"       # ARM; 32bit and 64bit

for PLATFORM in $PLATFORMS; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  _LDFLAGS=${LDFLAGS}
  BIN_FILENAME="${BINARY}-${GOOS}-${GOARCH}"
  if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
  if [[ "${GOOS}" != "linux" ]]; then _LDFLAGS="${LDFLAGS_OTHER}"; fi
  CMD="GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags \"${_LDFLAGS}\" -o ${BIN_FILENAME} ."
  echo "${CMD}"
  eval $CMD || FAILURES="${FAILURES} ${PLATFORM}"
done

# eval errors
if [[ "${FAILURES}" != "" ]]; then
  echo ""
  echo "${BINARY} build failed on: ${FAILURES}"
  exit 1
fi
