#!/bin/sh
set -eu
target=
old=
cleanup() {
  if [ -n "$old" ]; then docker rm -f "$old" >/dev/null 2>&1 || true; fi
  if [ -n "$target" ]; then docker rm -f "$target" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT
target=$(docker run -d --network none --entrypoint /bin/sh podtoast:test -c 'sleep 120')
old=$(docker run -d --pid="container:$target" --network="container:$target" --cap-add SYS_PTRACE ghcr.io/ikimpriv/podtoast:0.8.0 idle)
docker exec -d "$old" /bin/sh -c 'sleep 3'
started=$(date +%s)
# Shutdown can end the exec stream before the helper acknowledges success.
docker exec -i "$old" /bin/sh -c 'set -eu; umask 077; d=$(mktemp -d /tmp/podtoast-retire.XXXXXX); trap '"'"'rm -f "$d/helper"; rmdir "$d" 2>/dev/null || true'"'"' EXIT; cat > "$d/helper"; chmod 700 "$d/helper"; exec "$d/helper" retire --temporary' < build/podtoast || true
attempt=0
while [ "$(docker inspect -f '{{.State.Running}}' "$old")" = true ] && [ "$attempt" -lt 20 ]; do
  sleep 1
  attempt=$((attempt + 1))
done
test "$(docker inspect -f '{{.State.Running}}' "$old")" = false
test "$(docker inspect -f '{{.State.Running}}' "$target")" = true
test "$(( $(date +%s) - started ))" -ge 2
echo 'Legacy agent drained; shared-namespace application survived.'
