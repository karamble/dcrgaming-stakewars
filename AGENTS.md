# Project continuity

## Existing mainnet bond

Before changing financial recovery or user profiles, read
[the non-secret legacy recovery note](docs/legacy-bond-recovery.md). One 0.01 DCR
admission bond predates the bridge-controlled key scheme and is recovered
manually; its recovery material is backed up outside the repository. Never print,
commit, or copy its private key or seed into chat or assistant memory.

## Financial authority work

The game is untrusted. dcrpulse must control spending keys, independently verify
financial proposals, persist recovery data before offering a payment, and require
dashboard approval for financial operations. See
[the payment model](docs/payment-model.md).

No backward compatibility or legacy migration code is wanted. Recover the single
old bond manually from its separate backup; keep that exception out of the new
SDK/bridge financial protocol and application UI.

## SDK contract

- `Runtime.Fund` blocks until the payment is approved in dcrpulse, the request
  expires, or the caller's context ends. Run it as a job, not from a request
  handler (`internal/session/controller.go:313`).
- The `Settled` hook is never called. Poll `RefreshDeposits` until the stake
  reads `Check == "spent"`.
- `Outcome.Shares` are gross and must total the funded pot exactly. dcrpulse
  deducts the fee.
- There is no forfeiture, reclaim or refund API. Locked money is recovered by
  its owner in dcrpulse under Gaming then Recovery.

## Running the stack

`make acceptance` runs `simnet/run-financial-authority.sh`: two wallets, two
Bison Relay identities, two dcrpulse bridges, a played table and a cooperative
payout. See [docs/simnet.md](docs/simnet.md), including the host configuration
the money path requires.
