# Real StakeWars trace narrowing

This is local research, not an escrow, consensus protocol or on-chain verifier.
Run from the repository root:

```sh
make turntrace-check
make turntrace-demo
# Optional: longer prefix of a checked-in recording
go run ./research/turntrace/cmd -record pkg/replay/testdata/wide-six-squads.json -ticks 4096
```

The tool runs the actual `sim.Step` implementation, commits each canonical state
with SHA256, and builds a domain-separated indexed Merkle tree over those state
hashes. The commitment binds the trace length, simulation version, configuration
and prefix inputs. Padding cannot be opened as a real state.

It then constructs a deliberately false trace and narrows the differing final
result to adjacent states: both traces agree before the step and disagree after
it. Each midpoint is authenticated to its original root. The algorithm tolerates
traces that diverge and rejoin; it finds a disputed step, not necessarily the
first wrong step. It refuses changed contexts, lengths, indices, states and paths.

Measured fixture results:

| Trace | Ticks | Midpoint rounds | Largest serialized state | One opening |
|---|---:|---:|---:|---:|
| Actual two-player simulation | 512 | 9 | 148,032 bytes | 356 bytes |
| Actual wide six-player simulation | 512 | 9 | 394,452 bytes | 356 bytes |
| Synthetic maximum-length trace | 20,000 | 15 | Not measured | 516 bytes |

A midpoint round requests an opening from **each** party; the table gives the
size of one opening: 4-byte index + 32-byte state hash + Merkle siblings. Initial
and final endpoint openings are additional. There are no participant signatures,
transaction wrappers, fees or timeout witnesses in these sizes. **Rounds are not
transactions.** A trace with N ticks has N+1 state leaves, so tree proof depth can
exceed the number of midpoint rounds by one.

The actual simulation verifies the honest trace locally. A node does not run
`sim.Step`. Proving the identified tick wrong on-chain remains unimplemented.
The full wide state exceeds the stock transaction size limit; committing its
hash does not make its computation verifiable. State memory proofs and an
instruction-level verifier would be needed, or a different carefully specified
verification scheme.

Other explicit omissions:

- Authorization and canonical ordering of moves, skips and the initial state.
- Disclosure/data availability: a root does not provide the state or inputs.
- An executable on-chain bisection state machine or response deadlines.
- One-honest-versus-five challenge admission, arbitration and bounded fees.
- Payouts, refunds, key management, persistent restart state or wallet integration.
- A measured full-game/microinstruction trace or an affordable total fallback.

SHA256 state commitments here are **research-only**; production `replay.Hash`
uses BLAKE3 and is unchanged. The prefix is taken from an existing replay fixture,
not asserted to be an authenticated completed-turn message.

See [action enforcement direction](../../docs/action-enforcement-research.md).
