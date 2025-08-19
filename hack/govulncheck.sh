#!/bin/bash
# Source: https://github.com/tianon/gosu/blob/e157efb/govulncheck-with-excludes.sh
# Licensed under the Apache License, Version 2.0
# Copyright Tianon Gravi
set -Eeuo pipefail

exclude_arg=""
pass_args=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        -exclude)
            exclude_arg="$2"
            shift 2
            ;;
        *)
            pass_args+=("$1")
            shift
            ;;
    esac
done

if [[ -n "$exclude_arg" ]]; then
    excludeVulns="$(jq -nc --arg list "$exclude_arg" '$list | split(",")')"
else
    excludeVulns="[]"
fi

export excludeVulns

# Debug print
echo "excludeVulns = $excludeVulns"
echo "Passing args: ${pass_args[*]}"

if ! command -v govulncheck > /dev/null; then
    printf "govulncheck not installed"
    exit 1
fi

if out="$(govulncheck "${pass_args[@]}")"; then
	printf '%s\n' "$out"
	exit 0
fi

json="$(govulncheck -json "${pass_args[@]}")"

vulns="$(jq <<<"$json" -cs '
	(
		map(
			.osv // empty
			| { key: .id, value: . }
		)
		| from_entries
	) as $meta
	# https://github.com/tianon/gosu/issues/144
	| map(
		.finding // empty
		# https://github.com/golang/vuln/blob/3740f5cb12a3f93b18dbe200c4bcb6256f8586e2/internal/scan/template.go#L97-L104
		| select((.trace[0].function // "") != "")
		| .osv
	)
	| unique
	| map($meta[.])
')"
if [ "$(jq <<<"$vulns" -r 'length')" -le 0 ]; then
	printf '%s\n' "$out"
	exit 1
fi

filtered="$(jq <<<"$vulns" -c '
	(env.excludeVulns | fromjson) as $exclude
	| map(select(
		.id as $id
		| $exclude | index($id) | not
	))
')"

text="$(jq <<<"$filtered" -r 'map("- \(.id) (aka \(.aliases | join(", ")))\n\n\t\(.details | gsub("\n"; "\n\t"))") | join("\n\n")')"

if [ -z "$text" ]; then
	printf 'No vulnerabilities found.\n'
	exit 0
else
	printf '%s\n' "$text"
	exit 1
fi
