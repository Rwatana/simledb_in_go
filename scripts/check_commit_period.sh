#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
	printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
	exit 2
fi

base_sha="$1"
head_sha="$2"
subjects="$(git log --no-merges --format=%s "${base_sha}..${head_sha}")"
if [[ -z "$subjects" ]]; then
	echo "No Pull Request commit subjects were found."
	exit 1
fi

failed=false
while IFS= read -r subject; do
	if [[ "$subject" != *. ]]; then
		printf 'Commit subject must end with a period: %s\n' "$subject" >&2
		failed=true
	fi
done <<< "$subjects"

if [[ "$failed" == "true" ]]; then
	exit 1
fi

echo "All Pull Request commit subjects end with a period."
