#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

# Clean up any previous Kind clusters created by the test harness.
while IFS= read -r cluster; do
  kind delete cluster --name "$cluster"
done < <(kind get clusters | rg '^spegel-e2e-' || true)

export DOCKER_HOST="${DOCKER_HOST:-unix:///Users/xlsior/.docker/run/docker.sock}"
export INTEGRATION_TEST_STRATEGY="${INTEGRATION_TEST_STRATEGY:-fast}"
export INTEGRATION_TEST_KEEP_CLUSTER="${INTEGRATION_TEST_KEEP_CLUSTER:-1}"
export IMG_REF="${IMG_REF:-ghcr.io/spegel-org/spegel:$(git rev-parse --short HEAD)}"
export TEST_TIMEOUT="${TEST_TIMEOUT:-10m}"

GOOS=linux GOARCH=arm64 make build-image

cd "$ROOT/test/integration/kubernetes"
go test -v -timeout "$TEST_TIMEOUT" -count 1 -run TestKubernetes ./...
