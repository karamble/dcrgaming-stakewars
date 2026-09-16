#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
module_dir="$(realpath "$script_dir/..")"
dcrd_dir="$(realpath "$module_dir/../../../../decred/dcrd")"
fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/stakewars-crypto.XXXXXX")"
export GOCACHE="${GOCACHE:-/tmp/stakewars-go-cache}"
export CRYPTO_FIXTURE_DIR="$fixture_root"
go_cmd="${GO:-go}"

"$go_cmd" -C "$module_dir/../.." run -mod=readonly ./research/flightproof -export "$fixture_root/dcr-flight-proof"
"$go_cmd" -C "$module_dir" run -mod=readonly ./wotsserial -export "$fixture_root/dcr-wots-serial"
"$go_cmd" -C "$module_dir" run -mod=readonly ./memoryproof -export "$fixture_root/dcr-memory-proof"
"$go_cmd" -C "$module_dir" run -mod=readonly ./fixed -export "$fixture_root/dcr-covenant-probe"
"$go_cmd" -C "$module_dir" run -mod=readonly ./recursive -export "$fixture_root/dcr-recursive-probe"
"$go_cmd" -C "$module_dir" run -mod=readonly ./takegame \
    -export "$fixture_root/dcr-takegame-probe" \
    -checkpoint-export "$fixture_root/dcr-checkpoint-channel-probe" > "$fixture_root/takegame.log"

# Go's source overlay adds tests to the real dcrd package without writing to
# that checkout or replacing any implementation. Dependencies remain read-only.
python3 - "$script_dir/testdata" "$dcrd_dir" "$fixture_root/overlay.json" <<'PY'
import json
import pathlib
import sys

source, node, overlay = map(pathlib.Path, sys.argv[1:])
replacements = {}
for test in sorted(source.glob("*_test.go")):
    target = node / "internal/mempool" / ("stakewars_research_" + test.name)
    if target.exists():
        raise SystemExit(f"refusing to shadow existing source: {target}")
    replacements[str(target)] = str(test)
overlay.write_text(json.dumps({"Replace": replacements}))
PY

"$go_cmd" -C "$dcrd_dir" test -mod=readonly -overlay "$fixture_root/overlay.json" \
    -count=1 -v ./internal/mempool \
    -run '^(TestCovenantProbePolicy|TestCovenantTimeoutInputAge|TestExpiryOutputMaturityMainnetContext|TestTakeGamePolicy|TestWalletCheckpointChannelStockPolicy|TestWalletCheckpointChannelFirstSeenExitRace|TestWalletCheckpointLinkedFixtureAmounts|TestCertificatePipelineDepth|TestWOTSSerialStockPolicy)$'
printf 'Synthetic fixtures and source overlay: %s\n' "$fixture_root"
