#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/prepare-release.sh VERSION
  scripts/prepare-release.sh --check VERSION

Examples:
  scripts/prepare-release.sh v0.2.0
  scripts/prepare-release.sh --check v0.2.0
USAGE
}

mode="update"
if [[ "${1:-}" == "--check" ]]; then
  mode="check"
  shift
fi

tag="${1:-}"
if [[ -z "${tag}" || "${tag}" == "-h" || "${tag}" == "--help" ]]; then
  usage
  exit 1
fi

if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z._-]+)?$ ]]; then
  echo "error: VERSION must look like v0.2.0 or v0.2.0-rc.1" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readme="${repo_root}/README.md"

update_readme() {
  local file="$1"

  perl -0pi -e '
    BEGIN { $tag = shift @ARGV; }
    s{ocgo_v[0-9]+\.[0-9]+\.[0-9]+(?:[-.+][0-9A-Za-z._-]+)?_}{ocgo_${tag}_}g;
    s{github:ppanconi/ocgo/v[0-9]+\.[0-9]+\.[0-9]+(?:[-.+][0-9A-Za-z._-]+)?}{github:ppanconi/ocgo/${tag}}g;
  ' "${tag}" "${file}"
}

if [[ "${mode}" == "check" ]]; then
  tmp="$(mktemp)"
  trap 'rm -f "${tmp}"' EXIT
  cp "${readme}" "${tmp}"
  update_readme "${tmp}"

  if ! cmp -s "${readme}" "${tmp}"; then
    echo "error: README.md release examples are not in sync with ${tag}" >&2
    echo "run: scripts/prepare-release.sh ${tag}" >&2
    diff -u "${readme}" "${tmp}" || true
    exit 1
  fi

  echo "README.md release examples match ${tag}"
else
  update_readme "${readme}"
  echo "Updated README.md release examples to ${tag}"
fi
