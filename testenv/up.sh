#!/usr/bin/env bash
# Starts NetBox with the plugins of one profile and writes .env.<profile>
# holding NETBOX_SERVER_URL and NETBOX_API_TOKEN for the acceptance tests.
set -euo pipefail
cd "$(dirname "$0")"

profile=${1:?usage: up.sh <profile>, one of: $(ls profiles | sed 's/\.env$//' | tr '\n' ' ')}
# shellcheck disable=SC1090
source "profiles/${profile}.env"
engine=${CONTAINER_ENGINE:-$(command -v podman >/dev/null 2>&1 && echo podman || echo docker)}
name="nbp-${profile//./-}"
url="http://127.0.0.1:${PORT}"
admin_password="acctest-admin"

"$engine" build -q -t "localhost/${name}:latest" \
  --build-arg "NETBOX_IMAGE=${NETBOX_IMAGE}" \
  --build-arg "PLUGIN_REQUIREMENTS=${PLUGIN_REQUIREMENTS}" \
  -f Containerfile . >/dev/null

"$engine" network inspect "$name" >/dev/null 2>&1 || "$engine" network create "$name" >/dev/null

"$engine" run -d --name "${name}-pg" --network "$name" \
  -e POSTGRES_DB=netbox -e POSTGRES_USER=netbox -e POSTGRES_PASSWORD=acctest \
  docker.io/library/postgres:16-alpine >/dev/null
"$engine" run -d --name "${name}-redis" --network "$name" docker.io/library/redis:7-alpine >/dev/null
"$engine" run -d --name "$name" --network "$name" -p "127.0.0.1:${PORT}:8080" \
  -e DB_HOST="${name}-pg" -e DB_NAME=netbox -e DB_USER=netbox -e DB_PASSWORD=acctest \
  -e REDIS_HOST="${name}-redis" -e REDIS_DATABASE=0 \
  -e REDIS_CACHE_HOST="${name}-redis" -e REDIS_CACHE_DATABASE=1 \
  -e SECRET_KEY="acctest-acctest-acctest-acctest-acctest-acctest-acctest" \
  -e API_TOKEN_PEPPER_1="acctest-pepper-acctest-pepper-acctest-pepper-acctest-pepper" \
  -e SKIP_SUPERUSER=false -e SUPERUSER_NAME=admin -e SUPERUSER_PASSWORD="$admin_password" \
  -e SUPERUSER_EMAIL=admin@example.invalid -e ALLOWED_HOSTS='*' \
  "localhost/${name}:latest" >/dev/null

curl -sf -o /dev/null --retry 90 --retry-delay 5 --retry-all-errors --retry-max-time 600 "${url}/login/"

token=$(curl -sf -X POST "${url}/api/users/tokens/provision/" \
  -H 'Content-Type: application/json' -H 'Accept: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"${admin_password}\"}" |
  python3 -c 'import json,sys; d=json.load(sys.stdin); print("nbt_" + d["key"] + "." + d["token"] if d.get("version") == 2 else d["key"])')

cat >".env.${profile}" <<ENV
export NETBOX_SERVER_URL=${url}
export NETBOX_API_TOKEN=${token}
ENV
echo "${profile} ready at ${url}; source testenv/.env.${profile}"
