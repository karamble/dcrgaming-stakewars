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

The same command also tests a separate pre-signed-answer repair: all eight
linked rounds, destination tampering, missing backups, and contextual admission
of both answer and take branches. Neither the ladder nor the repair is used by
this game; payouts are cooperative and a refusal is answered by waiting out the
refund lock.
