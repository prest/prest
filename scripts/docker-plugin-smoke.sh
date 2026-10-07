#!/usr/bin/env bash
# Smoke-test the goreleaser "release" image: the entrypoint must rebuild the
# bundled plugins and prestd must load them (X-Hello-Middleware on responses).
#
# Usage: scripts/docker-plugin-smoke.sh <amd64|arm64> <path to linux prestd>
# GO_VERSION must be the toolchain that built prestd (default: local go).
set -euo pipefail

arch="${1:?arch (amd64 or arm64)}"
bin="${2:?path to a linux/${1} prestd binary}"
go_version="${GO_VERSION:-$(go env GOVERSION | sed 's/^go//')}"
name="prest-smoke-${arch}"
ctx="$(mktemp -d)"

cleanup() {
	docker rm -f "$name" "$name-pg" >/dev/null 2>&1 || true
	docker network rm "$name" >/dev/null 2>&1 || true
	rm -rf -- "${ctx:?}"
}
trap cleanup EXIT

# Same layout goreleaser dockers_v2 hands to buildx: $TARGETPLATFORM/prestd + extra_files.
[ -d vendor ] || go mod vendor
mkdir -p "$ctx/linux/$arch" "$ctx/etc" "$ctx/lib"
cp "$bin" "$ctx/linux/$arch/prestd"
cp -R etc/entrypoint.sh etc/plugin "$ctx/etc/"
cp -R lib/src "$ctx/lib/"
cp -R go.mod go.sum vendor Dockerfile "$ctx/"
cat >"$ctx/smoke.toml" <<'EOF'
pluginpath = "./lib"

[[pluginmiddlewarelist]]
file = "hello"
func = "Hello"
EOF

docker buildx build --load --platform "linux/$arch" --target release \
	--build-arg "GO_VERSION=$go_version" -t "$name" "$ctx"

docker network create "$name" >/dev/null
docker run -d --name "$name-pg" --network "$name" \
	-e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=prest postgres:17 >/dev/null
docker run -d --name "$name" --network "$name" --platform "linux/$arch" -p 127.0.0.1::3000 \
	-v "$ctx/smoke.toml:/app/smoke.toml:ro" \
	-e PREST_CONF=/app/smoke.toml -e PREST_PG_HOST="$name-pg" -e PREST_PG_PORT=5432 \
	-e PREST_PG_USER=postgres -e PREST_PG_PASS=postgres -e PREST_PG_DATABASE=prest \
	-e PREST_PG_SSL_MODE=disable -e PREST_JWT_DEFAULT=false \
	"$name" >/dev/null
port="$(docker port "$name" 3000/tcp | head -n1 | sed 's/.*://')"

# Plugin builds run inside the container, slowly under emulation.
ready=""
for _ in $(seq 1 120); do
	if curl -sf -o /dev/null "http://127.0.0.1:$port/_health"; then
		ready=1
		break
	fi
	if [ "$(docker inspect -f '{{.State.Running}}' "$name")" != "true" ]; then
		break
	fi
	sleep 5
done
if [ -z "$ready" ]; then
	echo "FAIL: prestd did not become healthy" >&2
	docker logs "$name" 2>&1 | tail -n 40 >&2
	exit 1
fi
if docker logs "$name" 2>&1 | grep -q 'vendor missing'; then
	echo "FAIL: image has no vendor/; plugins built without -mod=vendor" >&2
	exit 1
fi

# Plugin middleware wraps the CRUD stack, so probe a table route.
docker exec "$name-pg" psql -U postgres -d prest -qc 'CREATE TABLE smoke (id int)'
headers="$(curl -sf -D - -o /dev/null "http://127.0.0.1:$port/prest/public/smoke")"
if ! grep -qi '^X-Hello-Middleware:' <<<"$headers"; then
	echo "FAIL: X-Hello-Middleware missing; plugin did not load" >&2
	docker logs "$name" 2>&1 | grep -i plugin | tail -n 20 >&2
	exit 1
fi
echo "OK: linux/$arch plugin middleware loaded"
