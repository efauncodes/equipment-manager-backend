#!/usr/bin/env sh

set -eu

base_url="${BASE_URL:-}"
base_url="${base_url%/}"
admin_token="${ADMIN_SESSION_TOKEN:-}"
frontend_origin="https://equipment.sentient-octopus.dev"
run_full_flow="${RUN_FULL_FLOW:-false}"
tmp_dir="$(mktemp -d)"

cleanup() {
	status="$?"
	rm -rf "$tmp_dir"
	return "$status"
}
trap cleanup EXIT

if [ -z "$base_url" ]; then
	echo "set BASE_URL to the HTTPS staging URL" >&2
	exit 1
fi
if [ -z "$admin_token" ]; then
	echo "set ADMIN_SESSION_TOKEN from the protected staging mail flow" >&2
	exit 1
fi

case "$base_url" in
	https://*) ;;
	*) echo "BASE_URL must use https://" >&2; exit 1 ;;
esac

if [ "$run_full_flow" = "true" ]; then
	disposable_base_url="${DISPOSABLE_TEST_BASE_URL:-}"
	disposable_base_url="${disposable_base_url%/}"
	disposable_admin_token="${DISPOSABLE_ADMIN_SESSION_TOKEN:-}"
	disposable_member_id="${DISPOSABLE_MEMBER_ID:-}"
	disposable_member_token="${DISPOSABLE_MEMBER_SESSION_TOKEN:-}"
	test_run_id="${TEST_RUN_ID:-}"

	if [ -z "$disposable_base_url" ]; then
		echo "set DISPOSABLE_TEST_BASE_URL for RUN_FULL_FLOW=true" >&2
		exit 1
	fi
	if [ -z "$disposable_admin_token" ]; then
		echo "set DISPOSABLE_ADMIN_SESSION_TOKEN for RUN_FULL_FLOW=true" >&2
		exit 1
	fi
	if [ -z "$disposable_member_id" ]; then
		echo "set DISPOSABLE_MEMBER_ID for RUN_FULL_FLOW=true" >&2
		exit 1
	fi
	if [ -z "$disposable_member_token" ]; then
		echo "set DISPOSABLE_MEMBER_SESSION_TOKEN for RUN_FULL_FLOW=true" >&2
		exit 1
	fi
	if [ -z "$test_run_id" ]; then
		echo "set TEST_RUN_ID for RUN_FULL_FLOW=true" >&2
		exit 1
	fi

	case "$disposable_base_url" in
		http://*|https://*) ;;
		*) echo "DISPOSABLE_TEST_BASE_URL must use http:// or https://" >&2; exit 1 ;;
	esac
	if [ "$disposable_base_url" = "$base_url" ]; then
		echo "DISPOSABLE_TEST_BASE_URL must differ from BASE_URL" >&2
		exit 1
	fi
	case "$test_run_id" in
		''|*[!A-Za-z0-9._-]*) echo "TEST_RUN_ID must contain only letters, numbers, dot, underscore, or hyphen" >&2; exit 1 ;;
	esac
	if [ "${#test_run_id}" -gt 64 ]; then
		echo "TEST_RUN_ID must be at most 64 characters" >&2
		exit 1
	fi
fi

check_read_only() {
	target_url="$1"
	token="$2"
	label="$3"

	health_body="$(curl --fail --silent --show-error "$target_url/healthz")"
	if [ "$health_body" != '{"status":"ok"}' ]; then
		echo "$label /healthz returned an unexpected body" >&2
		exit 1
	fi
	ready_body="$(curl --fail --silent --show-error "$target_url/readyz")"
	if [ "$ready_body" != '{"status":"ok"}' ]; then
		echo "$label /readyz returned an unexpected body" >&2
		exit 1
	fi

	preflight_headers="$(mktemp "$tmp_dir/preflight.XXXXXX")"
	preflight_status="$(curl --silent --show-error --request OPTIONS \
		--dump-header "$preflight_headers" --output /dev/null --write-out '%{http_code}' \
		-H "Origin: $frontend_origin" \
		-H 'Access-Control-Request-Method: GET' \
		-H 'Access-Control-Request-Headers: Authorization' \
		"$target_url/api/v1/me")"
	if [ "$preflight_status" != "204" ]; then
		echo "$label CORS preflight returned HTTP $preflight_status, want 204" >&2
		exit 1
	fi
	grep -Fqi "Access-Control-Allow-Origin: $frontend_origin" "$preflight_headers"
	grep -Fqi 'Access-Control-Allow-Methods: GET, POST, PATCH, OPTIONS' "$preflight_headers"
	grep -Fqi 'Access-Control-Allow-Headers: Authorization, Content-Type, X-Request-ID' "$preflight_headers"
	grep -Fqi 'Access-Control-Max-Age: 600' "$preflight_headers"
	grep -Fqi 'Vary: Origin' "$preflight_headers"
	if grep -Fqi 'Access-Control-Allow-Credentials:' "$preflight_headers"; then
		echo "$label CORS preflight unexpectedly allows credentials" >&2
		exit 1
	fi

	curl --fail --silent --show-error "$target_url/api/v1/me" \
		-H "Authorization: Bearer $token" >/dev/null
	curl --fail --silent --show-error "$target_url/api/v1/members" \
		-H "Authorization: Bearer $token" >/dev/null
	curl --fail --silent --show-error "$target_url/api/v1/equipment" \
		-H "Authorization: Bearer $token" >/dev/null
}

check_read_only "$base_url" "$admin_token" "staging"

if [ "$run_full_flow" != "true" ]; then
	echo "remote readiness smoke test passed: $base_url"
	exit 0
fi

check_read_only "$disposable_base_url" "$disposable_admin_token" "disposable test target"

serial="remote-smoke-${test_run_id}-$(date -u +%Y%m%d%H%M%S)"

equipment_id="$(curl --fail --silent --show-error -X POST "$disposable_base_url/api/v1/equipment" \
	-H "Authorization: Bearer $disposable_admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"serial_number\":\"$serial\",\"type\":\"smoke-test-$test_run_id\",\"size\":\"M\",\"manufacturer\":\"remote-smoke-$test_run_id\"}" \
	| jq -er '.data.id')"

issuance_id="$(curl --fail --silent --show-error -X POST "$disposable_base_url/api/v1/issuances" \
	-H "Authorization: Bearer $disposable_admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"equipment_id\":\"$equipment_id\",\"member_id\":\"$disposable_member_id\"}" \
	| jq -er '.data.issuance.id')"

curl --fail --silent --show-error -X POST "$disposable_base_url/api/v1/issuances/$issuance_id/confirm" \
	-H "Authorization: Bearer $disposable_member_token" >/dev/null

return_id="$(curl --fail --silent --show-error -X POST "$disposable_base_url/api/v1/returns" \
	-H "Authorization: Bearer $disposable_admin_token" \
	-H 'Content-Type: application/json' \
	-d "{\"issuance_id\":\"$issuance_id\"}" \
	| jq -er '.data.return.id')"

curl --fail --silent --show-error -X POST "$disposable_base_url/api/v1/returns/$return_id/confirm" \
	-H "Authorization: Bearer $disposable_member_token" >/dev/null

history_count="$(curl --fail --silent --show-error "$disposable_base_url/api/v1/equipment/$equipment_id/history" \
	-H "Authorization: Bearer $disposable_admin_token" | jq -er '.data | length')"
if [ "$history_count" -lt 3 ]; then
	echo "disposable full smoke test returned too little history: $history_count" >&2
	exit 1
fi

echo "disposable full smoke test passed: $disposable_base_url (run $test_run_id)"
