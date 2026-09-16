#!/usr/bin/env bash
set -euo pipefail
here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
poker="$(realpath "$here/../../../../dcrpoker")"
run="$(mktemp -d "${TMPDIR:-/tmp}/stakewars-poker-exit.XXXXXX")"
python3 - "$here" "$poker" "$run/overlay.json" <<'PY'
import json, pathlib, sys
here,poker,out=map(pathlib.Path,sys.argv[1:])
target=poker/'pkg/escrow/stakewars_refusal_audit_test.go'
if target.exists(): raise SystemExit('Refusing to shadow existing source')
repair=poker/'pkg/escrow/stakewars_answer_repair_test.go'
if repair.exists(): raise SystemExit('Refusing to shadow existing source')
out.write_text(json.dumps({'Replace':{str(target):str(here/'testdata/refusal_test.go'),str(repair):str(here/'testdata/repair_test.go')}}))
PY
export POKER_EXIT_FIXTURES="$run"
export GOCACHE="${GOCACHE:-/tmp/stakewars-go-cache}"
"${GO:-go}" -C "$poker" test -mod=readonly -overlay "$run/overlay.json" -race -count=1 -v ./pkg/escrow -run '^TestStakeWarsPoker'
printf 'Source overlay: %s\n' "$run"

node="$(realpath "$poker/../../decred/dcrd")"
python3 - "$here" "$node" "$run/node-overlay.json" <<'PYCODE'
import json,pathlib,sys
here,node,out=map(pathlib.Path,sys.argv[1:])
target=node/'internal/mempool/stakewars_poker_exit_test.go'
if target.exists(): raise SystemExit('Refusing to shadow existing source')
out.write_text(json.dumps({'Replace':{str(target):str(here/'testdata/policy_test.go')}}))
PYCODE
"${GO:-go}" -C "$node" test -mod=readonly -overlay "$run/node-overlay.json" -count=1 -v ./internal/mempool -run '^TestStakeWarsPokerExitPolicy$'
