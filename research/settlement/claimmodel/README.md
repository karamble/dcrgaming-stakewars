# Bounded checkpoint-claim model

This is an executable model of candidate-verification control flow. It is **not
a cryptographic verifier, Decred contract, or complete settlement protocol**.
It does not authorize production funding.

Run from the StakeWars repository root:

```sh
python3 -m unittest discover -s research/settlement/claimmodel -p model.py -v
```

Python's standard library is sufficient.

## Modeled rules

- A participant from the fixed roster initiates one candidate at a time.
- A candidate names a strictly newer checkpoint within an agreed maximum.
- Verification advances through a bounded number of fragments. Only complete
  verification changes the certified checkpoint.
- A timeout abort restores the previous certified checkpoint and marks the
  claimant's attempt as exhausted at that checkpoint.
- Changing the proposed digest or sequence does not restore an attempt.
- Only accepting a fully verified, strictly newer checkpoint clears the failed
  attempts. This rule bounds resets by the maximum checkpoint number.

Each modeled progress edge strictly decreases a finite rank combining remaining
checkpoint advances, eligible claimants, and remaining verification steps.
The exhaustive six-participant fixture explores 7,105 states. With participant
0 holding a complete valid target certificate and making timely progress, its
3,265-state exploration has no maximal path ending before the target is verified.

## Counterexamples and checks

The tests intentionally reproduce two rejected designs:

1. **Reset attempts on abort:** an incomplete candidate returns to the identical
   initial state, allowing an infinite retry cycle.
2. **Advance the checkpoint from an unverified header:** a false maximum-sequence
   announcement makes a genuine certificate ineligible as a newer candidate.

Other tests check that an invalid 102-fragment candidate never advances the
certified checkpoint, that only its initiating participant loses an attempt,
and that changed sequences or outsider identities cannot bypass the attempt
limit.

## Assumptions and unmodeled requirements

- `valid` is an **ideal full-certificate predicate**. No WOTS, WOTS+, signature,
  hash, or proof implementation is supplied here.
- Initiation is assumed to authenticate the specified roster participant.
  Each step is assumed to enforce its exact index and immutable statement.
- Timeout eligibility is abstracted into an abort transition. The model does
  not implement block heights, CSV, reorgs, or transaction races.
- The honest participant retains its complete target certificate and associated
  game state, supplies proof data promptly, and obtains timely chain inclusion.
  Its timeout abort is excluded under those assumptions. If they fail, the
  no-dead-end claim does not apply.
- A signed state root does not provide the underlying game state. Durable
  transcript retention or an enforceable disclosure protocol remains necessary.
- The target is already available and verifiable. This model does not force a
  losing participant to provide a missing signature.
- Reaching the target means certificate verification, **not payout**. Candidate
  challenge windows, finalization, game execution and financial conservation
  are not modeled. A real protocol must give observers a fresh challenge window
  after complete verification; it must not require the whole multi-step proof
  to finish within an already elapsed nomination window.
- Script/resource budgets, fees, funding, data publication and claim collateral
  are outside the model. No production APIs or funds are used.

## Bounded does not mean affordable

Resetting attempts after each accepted checkpoint permits approximately
`participants × maximum checkpoint advances` candidate verifications. With six
participants, 1,000 advances and 102 fragments, the worst-case scale is roughly
612,000 verification transitions. At an illustrative 20,000 atoms per transition,
that is 122.4 DCR before claim/abort overhead. These are arithmetic estimates,
not measured verifier costs.

A global attempt cap might reduce cost, but safe handling of subsequently
revealed valid certificates would need further analysis. The model does not
select that policy. Multiple verification transactions can potentially be
pipelined; transaction count is not block count.
