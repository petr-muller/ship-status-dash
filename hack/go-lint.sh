#!/bin/bash

set -ex

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running golangci-lint..."
if command -v golangci-lint > /dev/null 2>&1; then
    golangci-lint "${@}"
else
    DOCKER=${DOCKER:-podman}

    if ! which "$DOCKER" > /dev/null 2>&1; then
        echo "golangci-lint not found and $DOCKER not available. Install golangci-lint or $DOCKER."
        exit 1
    fi

    # Check if running on Linux
    VOLUME_OPTION=""
    if [[ "$(uname -s)" == "Linux" ]]; then
        VOLUME_OPTION=":z"
    fi

    $DOCKER run --rm \
        --volume "${PROJECT_ROOT}:/workspace${VOLUME_OPTION}" \
        --workdir /workspace \
        quay.io/openshiftci/golangci-lint:v2.10.1 \
        golangci-lint "${@}"
fi
