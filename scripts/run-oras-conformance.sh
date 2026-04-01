#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
trust_status="$(cd "${root_dir}" && mise trust --show)"
is_root_untrusted=0
while IFS= read -r trust_line; do
  trust_path="${trust_line%%: *}"
  trust_state="${trust_line#*: }"
  normalized_path="${trust_path}"

  case "${trust_path}" in
    "~")
      normalized_path="${HOME}"
      ;;
    "~/"*)
      normalized_path="${HOME}/${trust_path:2}"
      ;;
  esac

  if [[ "${trust_state}" == "untrusted" && "${normalized_path}" == "${root_dir}" ]]; then
    is_root_untrusted=1
    break
  fi
done <<< "${trust_status}"

if [[ "${is_root_untrusted}" -eq 1 ]]; then
  echo "mise config for this repo/worktree is untrusted; run 'cd ${root_dir} && mise trust' (or trust ${root_dir}/mise.toml by equivalent means) before running this helper" >&2
  exit 1
fi

repository="${CUMASACH_ORAS_CONFORMANCE_REPOSITORY:-${CUMASACH_ARTIFACTORY_REPOSITORY:-}}"
username="${CUMASACH_ORAS_CONFORMANCE_USERNAME:-${ARTIFACTORY_USER:-}}"
password="${CUMASACH_ORAS_CONFORMANCE_PASSWORD:-${CUMASACH_ORAS_CONFORMANCE_PASS:-${CUMASACH_ORAS_CONFORMANCE_TOKEN:-${ARTIFACTORY_PASSWORD:-${ARTIFACTORY_PASS:-${ARTIFACTORY_API_TOKEN:-}}}}}}"
plain_http="${CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP:-${CUMASACH_ARTIFACTORY_PLAIN_HTTP:-}}"

if [[ -z "${repository}" ]]; then
  echo "set CUMASACH_ORAS_CONFORMANCE_REPOSITORY (legacy alias: CUMASACH_ARTIFACTORY_REPOSITORY)" >&2
  exit 1
fi

if [[ -z "${username}" ]]; then
  echo "set CUMASACH_ORAS_CONFORMANCE_USERNAME (legacy alias: ARTIFACTORY_USER)" >&2
  exit 1
fi

if [[ -z "${password}" ]]; then
  echo "set CUMASACH_ORAS_CONFORMANCE_PASSWORD (legacy aliases: CUMASACH_ORAS_CONFORMANCE_PASS, CUMASACH_ORAS_CONFORMANCE_TOKEN, ARTIFACTORY_PASSWORD, ARTIFACTORY_PASS, ARTIFACTORY_API_TOKEN)" >&2
  exit 1
fi

export CUMASACH_ORAS_CONFORMANCE_REPOSITORY="${repository}"
export CUMASACH_ORAS_CONFORMANCE_USERNAME="${username}"
export CUMASACH_ORAS_CONFORMANCE_PASSWORD="${password}"
if [[ -n "${plain_http}" ]]; then
  export CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP="${plain_http}"
fi

cd "${root_dir}/implementation/go"
mise exec -- go test ./internal/oci -run '^TestORASConformanceRoundTrip$' -count=1
