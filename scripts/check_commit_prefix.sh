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

prefixes_file="$(dirname "$0")/../hack/prefix.yml"
if [[ ! -f "$prefixes_file" ]]; then
	printf 'Allowed prefix configuration is missing: %s\n' "$prefixes_file" >&2
	exit 1
fi

allowed_prefixes=()
while IFS= read -r allowed_prefix; do
	[[ -z "$allowed_prefix" ]] && continue
	allowed_prefixes+=("$allowed_prefix")
done < <(sed -n 's/^  - //p' "$prefixes_file")

prefix_pattern='^\[([^][]+)\] .+'
failed=false
while IFS= read -r subject; do
	if [[ "$subject" =~ $prefix_pattern ]]; then
		prefix="${BASH_REMATCH[1]}"
	else
		printf 'Commit subject must start with an allowed bracketed prefix: %s\n' "$subject" >&2
		failed=true
		continue
	fi

	allowed=false
	for allowed_prefix in "${allowed_prefixes[@]}"; do
		if [[ "$prefix" == "$allowed_prefix" ]]; then
			allowed=true
			break
		fi
	done
	if [[ "$allowed" != "true" ]]; then
		printf 'Commit subject prefix [%s] is not allowed. Allowed prefixes: ' "$prefix" >&2
		printf '[%s] ' "${allowed_prefixes[@]}" >&2
		printf '\n' >&2
		failed=true
	fi
done <<< "$subjects"

if [[ "$failed" == "true" ]]; then
	exit 1
fi

echo "All Pull Request commit subjects use an allowed prefix."
