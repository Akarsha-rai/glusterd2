#!/usr/bin/env bash

## This scripts builds a GD2 binary and places it in the given path
## Should be called from the root of the GD2 repo as
## ./scripts/build.sh [<path-to-output-directory>]
## If no path is given, defaults to build

OUTDIR=${1:-build}
mkdir -p $OUTDIR

VERSION=$($(dirname $0)/pkg-version --full)
[[ -f VERSION ]] && source VERSION
GIT_SHA=${GIT_SHA:-$(git rev-parse --short HEAD || echo "undefined")}
GIT_SHA_FULL=${GIT_SHA_FULL:-$(git rev-parse HEAD || echo "undefined")}

REPO_PATH="github.com/gluster/glusterd2"
LDFLAGS="-X ${REPO_PATH}/version.GlusterdVersion=${VERSION} -X ${REPO_PATH}/version.GitSHA=${GIT_SHA}"
LDFLAGS+=" -B 0x${GIT_SHA_FULL}"

BIN=$(basename $(go list -f '{{.ImportPath}}'))

GOBUILD_TAGS=""
if [ "$PLUGINS" == "yes" ]; then
    GOBUILD_TAGS+="plugins "
    echo "Plugins Enabled"
else
    echo "Plugins Disabled"
fi

echo "Building $BIN $VERSION"

go build -ldflags "${LDFLAGS}" -o $OUTDIR/$BIN -tags "$GOBUILD_TAGS" || exit 1

echo "Built $BIN $VERSION at $OUTDIR/$BIN"
