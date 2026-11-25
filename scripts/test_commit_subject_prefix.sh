#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
test_repo="$(mktemp -d)"
trap 'rm -rf "$test_repo"' EXIT

git -C "$test_repo" init -q
git -C "$test_repo" config user.name "Commit subject test"
git -C "$test_repo" config user.email "commit-subject@example.invalid"
git -C "$test_repo" commit --allow-empty -qm "base"
base_sha="$(git -C "$test_repo" rev-parse HEAD)"

assert_accepts() {
	local subject="$1"
	git -C "$test_repo" commit --allow-empty -qm "$subject"
	local head_sha
	head_sha="$(git -C "$test_repo" rev-parse HEAD)"
	if ! (cd "$test_repo" && "$repo_root/scripts/check_commit_prefix.sh" "$base_sha" "$head_sha"); then
		printf 'Expected prefix check to accept: %s\n' "$subject" >&2
		exit 1
	fi
	base_sha="$head_sha"
}

assert_rejects() {
	local subject="$1"
	git -C "$test_repo" commit --allow-empty -qm "$subject"
	local head_sha
	head_sha="$(git -C "$test_repo" rev-parse HEAD)"
	if (cd "$test_repo" && "$repo_root/scripts/check_commit_prefix.sh" "$base_sha" "$head_sha"); then
		printf 'Expected prefix check to reject: %s\n' "$subject" >&2
		exit 1
	fi
	base_sha="$head_sha"
}

assert_accepts '[Backend] Store a block.'
assert_accepts '[CI/CD] Validate commit subjects.'
assert_accepts '[Docs] Explain commit subjects.'
assert_rejects 'Store a block.'
assert_rejects '[] Store a block.'
assert_rejects '[   ] Store a block.'
assert_rejects '[Backend]Store a block.'
assert_rejects '[Go] Store a block.'
assert_rejects '[Random] Store a block.'
assert_rejects '[backend] Store a block.'
