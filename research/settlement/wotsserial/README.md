# Pot-bound RFC 8391 chain-step experiment

This is research code with deterministic public fixtures. **Do not fund it.**
It demonstrates one recursive verification primitive, not a complete WOTS+
signature, checkpoint certificate, or recoverable payment contract.

## Demonstrated

A 131-byte state opening contains an arbitrary 32-byte statement, public seed,
RFC address (including signer/key slot, chain index and hash-step index), current
chain value, and a three-byte amount. Its BLAKE-256 commitment is embedded in the
current P2SH redeem script.

Each spend authenticates that opening, executes one RFC 8391 SHA2-256 chain step,
increments the exact hash-step index, preserves the statement and all other
address fields, and commits the next value into the successor script. The output
amount decreases by exactly 20,000 atoms. The covenant authenticates its actual
executing script and preserves the executable template.

The current fixture uses **699 script bytes, 254 of 255 non-push operations,
1,628 of 1,650 standard signature-script bytes, and a 1,737-byte transaction**.
Tests cover fifteen linked steps and reject changed statements, seeds, signer
slots, chain positions, skipped steps, values, amounts, expiry and reordered
fragments. High hash-step address bytes must be zero.

The stock-node policy overlay accepts two linked transactions, including an
unconfirmed child, with an independently seeded 3,000,000-atom funding output.
The 20,000-atom fee exceeds the 17,370-atom default relay minimum. The fake-chain
harness explicitly selects SHA256 verification, matching the already activated
`lnfeatures` agenda; its base flags alone omit agenda-dependent flags. See
[Decred's consensus vote archive](https://docs.decred.org/governance/consensus-rule-voting/consensus-vote-archive/)
and `dcrd/server.go`'s `standardScriptVerifyFlags` implementation. This is synthetic
chain/mempool validation, not live propagation or mining.

## Not demonstrated

- The initial hash-step digit is not derived from the statement here. Caller
  initialization must eventually enforce that relationship.
- The endpoint is not authenticated against a WOTS public key. No checksum,
  chain-to-chain progression, full signer, or multi-signer certificate is verified.
- There is no completion, exit, refund, challenge, or timeout branch. **The output
  after step 14 is a pending step-15 state that this program cannot spend.**
- There is no link from this partial computation to acceptance of a checkpoint.
- Only canonical positive three-byte ScriptNum amounts are supported. The fixed
  3,000,000-atom, fifteen-step fixture is valid; arbitrary stakes or full-signature
  fee reserves require a different amount representation.
- All steps use a fixed fee. No fee bumping, attempt budget, failure mask, restart
  recovery or production funding ceremony is implemented.

At one transaction per hash step, the checksum-constrained range is **45–990
steps per signature**, or up to **5,940 transactions for six signers**, before
endpoint authentication and checkpoint lifecycle overhead. At this fixture's fee
the six-signer upper bound is **1.188 DCR**. These are arithmetic bounds, not a
measured full-certificate construction. Practical batching and lifecycle design
remain open.

## Run

From the `research/settlement` module:

```sh
go test ./wotsserial
go run ./wotsserial -export /tmp/stakewars-wotsserial
bash policy/check.sh
```
