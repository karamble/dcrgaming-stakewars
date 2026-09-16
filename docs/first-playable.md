# First playable build — 2026-09-16

## Two game identities on one laptop

Build once:

```sh
make desktop
```

Run in separate terminals:

```sh
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player1" -settings bridge
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player2" -settings bridge
```

Connect each window to its own dcrpulse instance. Paste that instance's IP,
port, client certificate, private key and bridge certificate, then Connect and
Save. On first run StakeWars creates `dcrstakewars.conf` with documented defaults
(0600 permissions). Each directory contains its own config, `bridge.json`,
`controls.json`, `logs/dcrstakewars.log`, and `session/` (identity seed, SDK tables, payment records, signed turns and payout
receipts). Back up the entire directory. Reuse it to recover the same identity;
use a different directory to create a different identity. Never copy an identity
into a second concurrently running profile. SDK storage locks reject concurrent
ownership of the same session stores.

Subsequent launches can connect immediately:

```sh
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player1" -connect
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player2" -connect
```

Default directories use `dcrutil.AppDataDir("dcrstakewars", false)`:

| OS | Default directory |
| --- | --- |
| Linux/BSD | `~/.dcrstakewars` |
| macOS | `~/Library/Application Support/Dcrstakewars` |
| Windows | `%LOCALAPPDATA%\Dcrstakewars` (falls back to `%APPDATA%`) |

`--appdata` / `-A` and `--datadir` / `-datadir` select the entire profile.
`--configfile` / `-C` selects a different INI file. Defaults load first, the INI
file overrides defaults, and CLI options override the file. Existing config files
are preserved. `--bridge-config` and `--controls` override individual files without
changing the identity directory. Paths support environment variables and leading
`~`. To keep an earlier profile, explicitly pass `-datadir ~/.config/stakewars`;
no existing credentials or identity are moved or replaced automatically.

### Decred logging

The application uses `github.com/decred/slog`, with `SWAR` (desktop), `SESS`
(game sessions), `SDK` (SDK runtime), and `BRDG` (bridge transport) subsystems.
Logs go to stdout and `APPDATA/logs/dcrstakewars.log`, using
`github.com/jrick/logrotate/rotator`. Rotations are uncompressed numbered files;
the default is 1024 KB per file, retaining ten rotations.

```sh
/tmp/stakewars-bin/stakewars --appdata ~/.dcrstakewars-player1 --debuglevel SESS=debug,SDK=debug,BRDG=debug --connect
/tmp/stakewars-bin/stakewars --debuglevel show
```

`--logdir`, `--logsize` (KB), and `--maxlogfiles` override logging defaults and
can also be set in `dcrstakewars.conf`. Bridge certificates, private keys and
identity seeds are not included in startup logs. Do not put PEM text in CLI args.

## Table to battle

1. Create the StakeWars table in dcrpulse; choose the seats, buy-in, admission
   deadline and stake refund delay there. Send the invite into the group chat.
2. Each player clicks the chat invitation's Accept button. The SDK handles
   admission; approve the admission payment in dcrpulse.
3. StakeWars shows roster formation, confirmations and battlefield agreement.
   The seating block is `Until + 1`; this build requires six confirmations of
   that block before authorizing stake funding. Terrain, hazards, crates, wind
   and initial squad positions all derive from the same signed world agreement.
4. Set the payout/refund address in dcrpulse. Once the map is agreed, click
   **Request stake funding**, or press **F**, then approve the payment in dcrpulse.
5. When every stake is checked and payout addresses are known, press **Enter**
   in the table room to play. Only the active player controls a turn. Peers
   receive, authenticate, persist and independently replay the completed turn.
6. The verified terminal result creates the cooperative payout. Every player,
   including losers, signs the same transaction. The result screen distinguishes
   collecting signatures from payout broadcast. Broadcast is not confirmation.

Space charges/fires; J jumps; Shift+J backflips; A/D move; W/S aim (or adjust an
attached rope); Shift gives precision aim. Settings shows the complete key map.
Unfocused windows keep updating, which supports testing two instances locally.

The initial payment preset uses the configured stake and one refundable
0.01 DCR admission bond. It creates **no liveness or honesty bond**. The minimum
stake is 0.001 DCR to leave room for payout/refund fees. The cooperative settlement
fee is 0.001 DCR for the whole pot, deducted once from positive payouts. Admission
bonds have a 2016-block owner refund lock; the stake uses the invitation's lock
(minimum 288 blocks). These policies are bound by protocol version 3 and the
signed world manifest.

## Recovery and limitations

**Winnings require every player's signature.** A refusing signer can prevent
payout. Return to the table room and use **R / Refund stake** and **B / Refund
entry** once the respective locks mature. The screen reports remaining blocks;
the SDK independently rechecks scripts, values, unspent status and maturity
before signing. These refunds return each owner's original unspent deposit,
less fees. They do not award winnings. Tab cycles stored tables, including
recovery-only tables. Restore the same directory and reconnect to recover after
restart; configure a payout/refund address in dcrpulse again if requested.

A local signed turn is reserved durably before signing. Missing turns and SDK
financial announcements are requested again periodically. Accepted turns replay
from disk on restart; no peer-supplied final state authorizes a payout.

This is the **first cooperative playable implementation**, not a completed
adversarial consensus protocol or an independently audited real-money release.
A missing active player's signed batch currently stalls the match. Responding-peer
skip consensus remains unresolved under asynchronous partitions and is not
silently inferred from a timer. A connected idle player's countdown sends an
empty completed turn. The countdown continues while
menus or the lobby are open. Recover deposits if cooperation breaks down. Custom payout schemes
and configurable gameplay presets are also not exposed in this build.

All gameplay remains off chain. On-chain activity is limited to admission,
stake funding, cooperative payout, or owner refunds. The covenant experiments
remain documented under `research/settlement`; they are not used by this game.

## Isolated test tools

```sh
make multiplayer-demo   # two desktop windows, local mTLS bridge, simulated funds
make multiplayer-check  # two/six-player protocol, replay and owner-refund checks
make play               # existing local engineering arena
```

The demo never contacts a wallet or node. Its temporary profiles are printed at
startup; both windows are labelled SIMULATED FUNDS. It advances a fake chain to
map agreement, then waits for F in both windows before confirming fake stakes.
Press Enter in each table room. Closing a demo window stops the demo process.

Integration tests exercise the actual SDK and mTLS transport, real deterministic
shots, zero-share losing payout signers, restart replay, and owner refunds. They
use synthetic chain outputs, not live Bison Relay delivery or a live Decred wallet.

## Validation recorded for this build

- `make check`: race-enabled tests, Go vet, deterministic-simulation analyzer,
  and both desktop build variants.
- SDK runtime race suite: configurable settlement fee, payout-map concurrency,
  and the existing settlement/recovery tests.
- Configuration/logging tests: default-file creation and preservation, CLI-over-INI
  precedence, separate profiles, Unix permissions, path expansion, subsystem
  levels, and concurrent bounded rotation.
- Six-player lobby layout inspected in a generated fictional preview:
  `artifacts/stakewars-live-table.png`.
