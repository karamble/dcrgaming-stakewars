# Poker loser-refusal audit

Run `make poker-exit-check` from StakeWars, or `bash check.sh` here.

This runner overlays test files into the local sibling dcrpoker and dcrd
checkouts. It copies no implementation and changes neither checkout. Existing
Poker test helpers provide synthetic keys and deposits. The tests deliberately
retain successful attacks so that the actual guarantee remains visible.

The Poker tests run with the race detector. The node tests exercise stock
mempool checks with synthetic UTXOs, including real contextual CSV maturity.
These are not live-network or block-mining tests. Temporary overlays and exported
fixtures are kept in the printed `/tmp/stakewars-poker-exit.*` directory.

See [the audit](../../../docs/poker-exit-audit.md) for the transaction diagram,
source revision, amounts, timing, counterexamples and design decision.

The same command also tests an explicitly separate [pre-signed-answer repair](../../../docs/poker-ladder-repair.md):
all eight linked rounds, destination tampering, missing backups, and contextual
admission of both answer and take branches. This variant is not installed in Poker.
