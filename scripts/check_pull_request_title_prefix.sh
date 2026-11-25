#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
	printf 'Usage: %s <pull-request-title>\n' "$0" >&2
	exit 2
fi

title="$1"
prefixes_file="$(dirname "$0")/../hack/prefix.yml"
if [[ ! -f "$prefixes_file" ]]; then
	printf 'Allowed prefix configuration is missing: %s\n' "$prefixes_file" >&2
	exit 1
fi

prefix_pattern='^\[([^][]+)\] .+'
if [[ ! "$title" =~ $prefix_pattern ]]; then
	printf 'Pull Request title must start with an allowed bracketed prefix and a space: %s\n' "$title" >&2
	exit 1
fi

prefix="${BASH_REMATCH[1]}"
while IFS= read -r allowed_prefix; do
	if [[ "$prefix" == "$allowed_prefix" ]]; then
		echo "Pull Request title uses an allowed prefix."
		exit 0
	fi
done < <(sed -n 's/^  - //p' "$prefixes_file")

printf 'Pull Request title prefix [%s] is not allowed. Allowed prefixes: ' "$prefix" >&2
first=true
while IFS= read -r allowed_prefix; do
	if [[ "$first" == "true" ]]; then
		first=false
	else
		printf ', ' >&2
	fi
	printf '[%s]' "$allowed_prefix" >&2
done < <(sed -n 's/^  - //p' "$prefixes_file")
printf '\n' >&2
exit 1
