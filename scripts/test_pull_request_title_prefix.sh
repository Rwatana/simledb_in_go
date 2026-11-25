#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"

assert_accepts() {
	local title="$1"
	if ! (cd "$repo_root" && bash scripts/check_pull_request_title_prefix.sh "$title"); then
		printf 'Expected PR title prefix check to accept: %s\n' "$title" >&2
		exit 1
	fi
}

assert_rejects() {
	local title="$1"
	if (cd "$repo_root" && bash scripts/check_pull_request_title_prefix.sh "$title"); then
		printf 'Expected PR title prefix check to reject: %s\n' "$title" >&2
		exit 1
	fi
}

assert_accepts '[Backend] Validate subject scopes'
assert_accepts '[CI/CD] Add a title check'
assert_accepts '[Docs] Explain title prefixes.'
assert_rejects 'Validate subject scopes'
assert_rejects '[Go] Add implementation'
assert_rejects '[Unknown] Add a title check'
assert_rejects '[Backend]'
assert_rejects '[Backend]No space'
