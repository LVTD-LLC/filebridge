#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
publish_workflow="$repo_root/.github/workflows/publish.yml"

is_pinned_action_ref() {
	action_ref="$1"
	case "$action_ref" in
	./*) return 0 ;;
	*@*) ;;
	*) return 1 ;;
	esac
	action_sha="${action_ref##*@}"
	[ "${#action_sha}" -eq 40 ] &&
		[ -z "$(printf '%s' "$action_sha" | tr -d '0123456789abcdefABCDEF')" ]
}

for mutable_ref in owner/action@main owner/action@v4 owner/action@abc123; do
	if is_pinned_action_ref "$mutable_ref"; then
		printf 'mutable action ref passed pin validation: %s\n' "$mutable_ref" >&2
		exit 1
	fi
done

action_refs="$(
	find "$repo_root/.github/workflows" -type f \( -name '*.yml' -o -name '*.yaml' \) \
		-exec awk '
			/^[[:space:]]*uses:[[:space:]]*/ {
				sub(/^[[:space:]]*uses:[[:space:]]*/, "")
				sub(/[[:space:]#].*$/, "")
				print
			}
		' {} +
)"
while IFS= read -r action_ref; do
	if [ -n "$action_ref" ] && ! is_pinned_action_ref "$action_ref"; then
		printf 'GitHub Action is not pinned to a full commit SHA: %s\n' "$action_ref" >&2
		exit 1
	fi
done <<EOF
$action_refs
EOF

for runner in ubuntu-latest ubuntu-24.04-arm macos-15-intel macos-15; do
	if ! grep -Fq "runner: $runner" "$publish_workflow"; then
		printf 'missing native CLI release runner: %s\n' "$runner" >&2
		exit 1
	fi
done

if ! grep -Fq 'name: Smoke-test CLI archive' "$publish_workflow"; then
	printf 'release workflow does not smoke-test final CLI archives\n' >&2
	exit 1
fi

if ! grep -Eq 'permissions:[[:space:]]*$' "$publish_workflow" ||
	! grep -Fq 'contents: read' "$publish_workflow"; then
	printf 'release workflow does not default to read-only contents permission\n' >&2
	exit 1
fi

printf 'CLI release contract tests passed\n'
