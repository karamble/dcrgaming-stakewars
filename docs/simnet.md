# The simnet acceptance harness

`simnet/run-financial-authority.sh` builds the stack from source, runs it on a
private chain, and plays a funded table from invitation to payout.

```sh
make acceptance
```

State lives under `/tmp/stakewars-simnet-acceptance` and is removed on the next
run. Nothing touches mainnet.

## Requirements

- `go`, `curl`, `jq`, `docker`, `ss`.
- Sibling source trees, all built by the harness: `karamble/dcrgaming-sdk`,
  `karamble/dcrpulse`, `karamble/brclientd`, `decred/dcrd`, `decred/dcrwallet`,
  `companyzero/bisonrelay`. Override with `SDK_REPO`, `PULSE_REPO`,
  `BRCLIENT_REPO`, `DCRD_REPO`, `WALLET_REPO`, `BR_REPO`, or `SRC_ROOT`.
- A container image for dcrpulse (`PULSE_IMAGE`, default `alpine:3.22`).
  dcrpulse resolves paths against a fixed `/app-data`, so it runs in a container
  with `--network host`.
- Free ports: 19443, 19555, 19556, 19557, 19567, 19680, 19681, 19690, 19691,
  19801, 19802. The harness exits if they are in use.

## Switches

| Variable | Effect |
|---|---|
| `SKIP_BUILD=1` | Reuse the previous run's binaries. |
| `KEEP_RUNNING=1` | Leave the stack up on failure; prints dashboard URLs, container names and the run directory. |
| `PULSE_LOG_LEVEL=debug` | dcrpulse log level. Its gaming subsystem reports most refusals at debug only. |
| `MINE_WHILE_WAITING=n` | Blocks a single wait may mine. Default 6. |
| `STAKEWARS_SIMNET_ROOT` | Where run state lives. |

## Phases

1. Build all sources.
2. Start `brserver` and two `brclientd` identities, introduce them, create one
   group chat.
3. Start two dcrpulse bridges, each with its own wallet, dashboard session and
   issued game credential.
4. Create a table that never fills, let admission lapse, and recover the
   admission bond unilaterally after its lock matures.
5. Create a two-player table: both seats accept, both bonds approved, chain
   passes the admission deadline, seats drawn, both stakes funded and approved,
   match played with both peers asserting an identical log head after each move.
6. Settle: both bridges derive the same payout id, both operators approve, the
   transaction is broadcast and confirmed.
7. Recover the bond from phase 4.

## Host configuration

Each of these fails silently — the bridge answers one opaque string, or nothing
happens. The harness sets them up; a real deployment needs the same.

- **`txindex=1` on dcrd.** Approving a payout does not broadcast it. A reconcile
  pass every 30 seconds does, and it first calls `getrawtransaction`. Without
  the index that call fails with an error dcrpulse cannot read as "not found",
  so a payout signed by both seats is never sent. It stays at `publishing` and
  logs nothing above debug.
- **dcrwallet started `--noinitialload`.** dcrpulse caches whether a wallet can
  sign from its own open response. Its open path short-circuits on an
  already-loaded wallet, so the flag is never written and every gaming financial
  call fails with `financial authority is unavailable`. Letting dcrpulse open
  the wallet also puts it in charge of the sync.
- **Wallet reporting `synced` before any spend.** Approving earlier returns
  `broadcast outcome unknown; bridge will reconcile the recorded transaction`.
  Reconcile it; do not re-approve.
- **A bound gaming account that is not a mixing account.** Funding change leaves
  a mixed account and the spend is refused.
- **The Bison Relay identity at `/app-data/brclientd`.** dcrpulse reads the RPC
  certificates from that fixed path; `BRCLIENTD_DATA_DIR` covers only embeds,
  downloads and logs. Mounted elsewhere, the bridge has no identity.

## Blocks and heights

Simnet mints a block only on request, so waits that depend on a confirmation
mine for themselves, slowly and capped by `MINE_WHILE_WAITING`. Play carries a
move deadline, so waits that watch a peer rather than the chain do not mine.

Admission closes at the invitation's `until`. Seats are drawn from the hash of
the next block, so nothing is seated until the chain passes it.
