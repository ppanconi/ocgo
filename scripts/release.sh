#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/release.sh VERSION

Example:
  scripts/release.sh v0.2.0

This updates README release examples, commits that change if needed, creates the
release tag, and pushes the current branch plus the tag.
USAGE
}

tag="${1:-}"
if [[ -z "${tag}" || "${tag}" == "-h" || "${tag}" == "--help" ]]; then
  usage
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z._-]+)?$ ]]; then
  echo "error: VERSION must look like v0.2.0 or v0.2.0-rc.1" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  echo "error: tag ${tag} already exists" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree has uncommitted changes" >&2
  echo "commit or stash them before running release.sh" >&2
  exit 1
fi

current_branch="$(git branch --show-current)"
if [[ -z "${current_branch}" ]]; then
  echo "error: not on a branch" >&2
  exit 1
fi

scripts/prepare-release.sh "${tag}"

if ! git diff --quiet -- README.md; then
  git add README.md
  git commit -m "Prepare release ${tag}"
fi

scripts/prepare-release.sh --check "${tag}"

git tag "${tag}"
git push origin "${current_branch}" "${tag}"

echo "Released ${tag}"
