#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

repository="${CUMASACH_ORAS_CONFORMANCE_REPOSITORY:-${CUMASACH_ARTIFACTORY_REPOSITORY:-}}"
username="${CUMASACH_ORAS_CONFORMANCE_USERNAME:-${ARTIFACTORY_USER:-}}"
password="${CUMASACH_ORAS_CONFORMANCE_PASSWORD:-${CUMASACH_ORAS_CONFORMANCE_PASS:-${CUMASACH_ORAS_CONFORMANCE_TOKEN:-${ARTIFACTORY_PASSWORD:-${ARTIFACTORY_PASS:-${ARTIFACTORY_API_TOKEN:-}}}}}}"
plain_http="${CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP:-${CUMASACH_ARTIFACTORY_PLAIN_HTTP:-}}"

: "${repository:?set CUMASACH_ORAS_CONFORMANCE_REPOSITORY}"
: "${username:?set CUMASACH_ORAS_CONFORMANCE_USERNAME}"
: "${password:?set CUMASACH_ORAS_CONFORMANCE_PASSWORD}"

export CUMASACH_ORAS_CONFORMANCE_REPOSITORY="${repository}"
export CUMASACH_ORAS_CONFORMANCE_USERNAME="${username}"
export CUMASACH_ORAS_CONFORMANCE_PASSWORD="${password}"
if [[ -n "${plain_http}" ]]; then
  export CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP="${plain_http}"
fi

cd "${root_dir}/implementation/go"
go test ./internal/oci -run '^TestORASConformanceRoundTrip$' -count=1
