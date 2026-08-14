#!/usr/bin/env sh

set -eu

compose_file="${COMPOSE_FILE:-docker-compose.yml}"
backend_port="${BACKEND_PORT:-8080}"

at_exit() {
	status=$?
	docker compose -f "$compose_file" down --remove-orphans >/dev/null 2>&1 || true
	exit "$status"
}
trap at_exit EXIT INT TERM

docker compose -f "$compose_file" up -d --build

i=0
while [ "$i" -lt 30 ]; do
	if curl --fail --silent --show-error "http://127.0.0.1:${backend_port}/healthz" >/dev/null; then
		echo "Smoke test passed: /healthz is reachable."
		exit 0
	fi
	i=$((i + 1))
	sleep 1
done

echo "Smoke test failed: /healthz did not become reachable." >&2
exit 1
