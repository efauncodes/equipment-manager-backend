#!/usr/bin/env sh

set -eu

base_url="${BASE_URL:?set BASE_URL to the HTTPS staging URL}"
base_url="${base_url%/}"
admin_token="${ADMIN_SESSION_TOKEN:?set ADMIN_SESSION_TOKEN from the protected staging mail flow}"

case "$base_url" in
	https://*) ;;
	*) echo "BASE_URL must use https://" >&2; exit 1 ;;
esac

curl --fail --silent --show-error "$base_url/healthz" >/dev/null
curl --fail --silent --show-error "$base_url/api/v1/me" \
	-H "Authorization: Bearer $admin_token" >/dev/null
curl --fail --silent --show-error "$base_url/api/v1/members" \
	-H "Authorization: Bearer $admin_token" >/dev/null
curl --fail --silent --show-error "$base_url/api/v1/equipment" \
	-H "Authorization: Bearer $admin_token" >/dev/null

if [ "${RUN_FULL_FLOW:-false}" != "true" ]; then
	echo "remote readiness smoke test passed: $base_url"
	exit 0
fi

member_id="${MEMBER_ID:?set MEMBER_ID for RUN_FULL_FLOW=true}"
member_token="${MEMBER_SESSION_TOKEN:?set MEMBER_SESSION_TOKEN for RUN_FULL_FLOW=true}"
serial="remote-smoke-$(date -u +%Y%m%d%H%M%S)"

equipment_id="$(curl --fail --silent --show-error -X POST "$base_url/api/v1/equipment" \
	-H "Authorization: Bearer $admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"serial_number\":\"$serial\",\"type\":\"smoke-test\",\"size\":\"M\",\"manufacturer\":\"remote-smoke\"}" \
	| jq -er '.data.id')"

issuance_id="$(curl --fail --silent --show-error -X POST "$base_url/api/v1/issuances" \
	-H "Authorization: Bearer $admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"equipment_id\":\"$equipment_id\",\"member_id\":\"$member_id\"}" \
	| jq -er '.data.issuance.id')"

curl --fail --silent --show-error -X POST "$base_url/api/v1/issuances/$issuance_id/confirm" \
	-H "Authorization: Bearer $member_token" >/dev/null

return_id="$(curl --fail --silent --show-error -X POST "$base_url/api/v1/returns" \
	-H "Authorization: Bearer $admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"issuance_id\":\"$issuance_id\"}" \
	| jq -er '.data.return.id')"

curl --fail --silent --show-error -X POST "$base_url/api/v1/returns/$return_id/confirm" \
	-H "Authorization: Bearer $member_token" >/dev/null

history_count="$(curl --fail --silent --show-error "$base_url/api/v1/equipment/$equipment_id/history" \
	-H "Authorization: Bearer $admin_token" | jq -er '.data | length')"
if [ "$history_count" -lt 3 ]; then
	echo "remote full smoke test returned too little history: $history_count" >&2
	exit 1
fi

echo "remote full smoke test passed: $base_url"
