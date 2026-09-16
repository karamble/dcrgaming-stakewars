# Settlement cryptography research

> Research record. The approved product model is now [cooperative payouts with
> independent owner refunds](payment-model.md). Enforced winnings against a
> refusing signer and on-chain gameplay verification are not release requirements.

Research date: 2026-09-16. Target: existing Decred mainnet rules, no trusted
referee and no honest-majority assumption among the players. This is an
experimental construction, not an audited escrow or permission to enable paid
matches. Production settlement remains gated.

Reproduce the preserved experiments with `make crypto-check` and
`make crypto-policy-check`. Code and evidence boundaries are described in
[`research/settlement`](../research/settlement/README.md).

## New result: transaction constraints without a new opcode

The local dcrd checkout supports `OP_CAT`, byte slicing, BLAKE-256 and Decred
Schnorr verification. Together these permit a small experimental script to
reconstruct a transaction signature hash and check it against the actual
spending transaction. It can then constrain the transaction's outputs.

This corrects an overly broad inference from the absence of a *native*
covenant opcode: absence of that opcode does not establish that transaction
constraints are impossible. A covenant is simply a spending condition that
restricts where the funds can go next.

### Construction and security argument

Decred Schnorr verifies `R = sG + eQ`, where
`e = BLAKE256(R.x || transactionSignatureHash)`, and requires R to have even Y.
Choose the public, synthetic points `Q = -G` and `R = G`. The required scalar
is then `s = 1 + e`.

The script calculates `e` from the supplied transaction preimage. It accepts
only a final hash byte in 1..126, increments that byte and concatenates the
result with the first 31 bytes. This implements `e + 1` without 256-bit Script
arithmetic or carry. The transaction builder searches a permitted transaction
field until that condition holds. The script constructs the complete Schnorr
signature and verifies it using `OP_CHECKSIGALT` against the real transaction.

If verification succeeds, the curve equation and even-Y check require the
claimed and actual challenges to agree modulo the group order. The accepted
scalar encoding and no-carry restriction rule out an arithmetic wrap in this
construction. Under the hash collision-resistance assumptions, that binds the
claimed signature-hash preimage to the actual spending transaction.

Decred's `SIGHASH_ALL` message is:

```
BLAKE256(LE32(SIGHASH_ALL) || BLAKE256(prefix) || BLAKE256(scriptWitness))
```

The script must reconstruct the exact consensus serialization, including
counts, versions, lengths and the script code of the executing input. A
fixed-output experiment constrains the output serialization. A recursive
experiment authenticates the supplied current redeem script through the
script-witness hash, preserves its executable suffix, changes only its state
prefix, and constrains the successor to the resulting P2SH script.

**The synthetic key is public and provides no player authentication.** A real
player signature is a separate requirement for an authorized move. Never use
this deliberately known scalar as a wallet key. Never use a player's key or
nonce for this synthetic signature operation.

### Limits of the result

Passing the script engine proves the tested spending conditions, not full
transaction or block acceptance. Relay policy, input existence and values,
fees, relative maturity, reorganizations and adversarial transaction conflicts
need separate tests. Freshly re-signing and rebuilding malicious spends is
essential: merely changing an already-signed transaction would only test
ordinary signature protection.

The experiment does not yet verify StakeWars physics, authoritative off-chain turn
history, data availability or six-player dispute rights. It does not justify
using experimental outputs with real money.

### Relation to Poker's ladder

Poker's existing bond ladder uses pre-signed accusation/answer transactions to
constrain each permitted destination and to start a fresh response window. Its
answer proves presence; settlement still needs every stake owner's signature.
The new primitive can enforce a changing successor state directly in Script,
without obtaining setup signatures for every possible continuation. It builds
on the same challenge/response structure but establishes a different property.

The preserved take-one-or-two game computes either player's winning payout,
allows an absent player's turn to be skipped, and needs no loser signature at
settlement. Its moves, winner payouts, skip and bounded draw pass stock
`ProcessTransaction` with synthetic UTXOs and mainnet policy. This establishes
a small **on-chain** game, not StakeWars' off-chain settlement protocol.

### Remaining rollback attack

The actual linked-transaction counterexample is:

```
A takes 1 -> B takes 1 -> A takes 1 -> A wins
A takes 1 -> B takes 2             -> B wins
```

Both B spends consume the same previous output and satisfy the covenant. B can
publish the second branch after the first history was exchanged off-chain.
The covenant cannot currently distinguish an earlier agreed history from a
later legally signed alternative. A final winning transaction remains tied to
the parent transaction of its own branch. A challenge/revocation/checkpoint
mechanism is still needed before treating BR history as irreversible.

### Bounded checkpoint recovery now demonstrated

An additional prototype supplies that mechanism for four pre-enumerated,
fully endorsed snapshots. Each player commits an independent secret for each
exact snapshot. Both openings form a certificate which Script can verify
independently of the spending outpoint.

The complete linked flow passes the script engine and stock contextual mempool
tests:

1. Funding enters a **fresh** claim, opening a new CSV window even when the
   funding output is old. Direct funding exit is forbidden.
2. A complete certificate for a strictly newer snapshot replaces the claim,
   preserving the right state and explicit fee schedule.
3. After the new window matures, the claim exits into the exact game covenant.
4. A resumes after the endorsed B1 move and signs the winning move alone. B2
   is not a valid spend of that resumed state.

The claim redeem script is 462 bytes; update/exit signature scripts are about
1,439/1,375 bytes. Each path to a snapshot produces the same game funding amount.
Jumping past snapshots consumes the corresponding reserved fees.

Necessary conditions, retained as tests:

* Every funder must already possess and durably retain the complete initial
  certificate before releasing funding signatures. Otherwise peer refusal can
  lock the root output.
* The honest watcher must confirm an update before stale exit maturity. Once
  both spends are eligible, stock mempool policy accepts the first and rejects
  the other as a double spend. Sequence freshness is not replacement authority.
* An informal acknowledgment or partial certificate does not establish finality.
* Four predefined snapshots are not a scalable encoding of unpredictable
  StakeWars hashes. This is not a general signature scheme.

The bare-game rollback counterexample therefore remains valid. The added
checkpoint layer addresses its bounded, fully endorsed case under the stated
monitoring and funding conditions.

An independent review caught unconstrained expiry in an early prototype;
the preserved scripts now require zero expiry. They also restrict real actor
signatures to `SIGHASH_ALL`. The known synthetic scalar cannot authorize an
actor's move. The contextual tests separately check CSV input age and the
256-block maturity consequence of expiring parents.

## Timeout semantics and a Decred-specific trap

The useful candidate has two spending branches:

* **Move:** the active player authorizes an enforced legal transition.
* **Skip:** after a CSV response window, anyone can execute the exact empty-turn
  transition, without the absent player's signature.

Both must advance a bounded game state. A heartbeat cannot reset the deadline
while avoiding the move or payout obligation. The first confirmed spend
determines the canonical continuation; a legal move can still win that race
after a skip becomes eligible. This is an earliest-skip rule, not a strict
latest-response deadline.

The normal BR countdown cannot prove absence on chain. A disputed match needs
chain monitoring and an explicit chain response window; later speculative
turns may need to pause or roll back. These are candidate protocol semantics,
not a silent change to StakeWars' approved game rules.

Decred transaction expiry could make response and timeout height ranges
disjoint, but outputs of a transaction carrying expiry must mature for
`CoinbaseMaturity`, currently 256 mainnet blocks. Expiry on every intermediate
step would therefore impose substantial delays. Do not use it as an assumed
free upper-deadline primitive or as a harmless nonce-grinding field.

## What still has to be established

1. **Correct unilateral StakeWars payout:** the bounded on-chain take game now
   demonstrates this property for its own rules. Extend it to irreversible
   off-chain history and StakeWars' rules without introducing a loser veto.
2. **Canonical inputs:** bind actions, actor identity, turn index and skips to
   one enforceable history. Independently replaying different authentic logs
   does not establish a common result.
3. **Data availability:** a committed action hash is insufficient if an honest
   challenger cannot obtain its contents. Publish bounded data or provide an
   enforceable disclosure procedure before treating the input as final.
4. **Scalable execution verification:** reduce a disputed execution to small
   on-chain checks, with exact commitments to the program and initial state.
   Pre-signing every complete game continuation is not a practical solution.
5. **One honest player against five:** a colluding claimant and first challenger
   must not exclude another player's challenge. Rejecting a false assertion
   must preserve a path to the correct payout, not merely burn a bond.
6. **Bounded cost and exit:** bound turns, claims, response rounds, script sizes,
   fee reserves and delay. Include refused setup, withheld data, stale states,
   restart, reorg and mempool-pinning attacks. State chain-inclusion and watcher
   assumptions explicitly.

## Remaining certificate scaling research

Hash-based signatures are a possible route to arbitrary checkpoint statements.
With 32-byte elements and Winternitz parameter 16, 67 elements consume 2,144
bytes per signer before covenant overhead; six consume 12,864 bytes. A serial
verifier could carry one statement and exact progress indices across constrained
spends, but its complete script budgets and resistance to repeated bad claims
are unproven. WOTS+ requires its address-dependent chaining construction, not
merely repeated hashes. [RFC 8391](https://www.rfc-editor.org/rfc/rfc8391)

A contextual test accepts 102 ordinary unconfirmed transactions parent-first
without advancing chain height. A hypothetical 102-fragment verifier would not
inherently require 102 blocks; the test establishes neither that verifier nor
mining behavior. A single 103-input transaction is unsuitable for the current
naive introspection encoding: its prefix alone is 4,271 bytes, above the
2,048-byte stack-element limit.

## Arbitrary-digest and execution-proof experiments

The isolated `research/settlement/wots` package implements RFC 8391 SHA2-256
WOTS+ reference signing and verification, plus actual Decred Script chain-step
emitters. A fixture verifies all 67 chains (including checksum) through 435
individual script executions. This is not one whole-signature covenant: the
surrounding transaction protocol must authenticate the message, key, indices,
endpoints and completion. Each secret key may sign only one message.

The dynamic step measures 391 bytes and 141 non-push operations before its
wrapper. Exact verification work ranges from 45 to 990 chain steps per signer.
One transaction per step therefore reaches 5,940 transactions for six signers,
before initialization, endpoint proofs and recovery. At an illustrative 20,000
atoms per transaction that is 1.188 DCR for just those steps. The previous
102-fragment figure was a batching hypothesis; it is not an implemented budget.

`research/settlement/wotsserial` wraps one genuine WOTS+ chain step in a
recursive pot constraint. The committed opening binds the message, public seed,
address, current hash and amount; the output must advance one step with the
computed hash and fixed fee. After independent review added the missing high
address-byte check, the fixture measures 699 redeem-script bytes, 254 of 255
operations, and 1,628 of 1,650 permitted signature-script bytes. Fifteen linked
steps execute in the script engine; stock mempool tests accept a parent and its
unconfirmed child with the activated SHA256 verification flag. This is close to the resource ceiling. There is no enrolled
public-key endpoint proof, whole-signature completion, abort or payout branch;
the terminal chain state cannot release funds. Its three-byte amount encoding
also does not cover a full multi-player verification fee reserve. Never fund it.

`research/settlement/memoryproof` checks a separate execution primitive: a
19-sibling proof authenticates one cell from a fully constructed 524,288-cell
memory, and script rejects an incorrect claimed result for a fixed read-plus-add
instruction. Correct arithmetic cannot trigger this fraud branch. A covenant
forces its exact beneficiary output. The redeem script is 425 bytes and the
input script is 1,183 bytes; stock mock-chain mempool admission passes. This
establishes neither an authoritative trace nor a general virtual machine.

`research/settlement/claimmodel` explores 7,105 abstract states and reproduces
abort-reset and unverified-sequence starvation bugs. The bounded alternative
preserves the old certified checkpoint during partial verification, authenticates
claimants, records failed attempts and only resets the attempt mask after a
fully verified sequence advance. Its one-honest-versus-five model assumes the
honest participant already holds a complete target certificate and durable data.
It does not manufacture withheld signatures. Actual scripts, inclusion deadlines,
fresh challenge windows and funding reserves remain separate obligations.

**Practical verdict:** current Script supports more useful verification than a
simple multisig analysis suggests. These results do not yet provide an affordable,
complete, uncooperative-loser settlement protocol for StakeWars. Production
funding remains gated. Next requirements are complete certificate sequencing,
interruption recovery, cheaper bounded claims, and an authoritative execution
history with data availability.

## Source basis

Local code inspected: dcrd `b9634e01`, dcrwallet `c0fee6b5`, and the working SDK
checkout. Relevant dcrd source locations are `txscript/sighash.go`,
`txscript/opcode.go`, `dcrec/secp256k1/schnorr/signature.go`,
`internal/mempool/policy.go`, and `internal/blockchain/validate.go`.

Research antecedents, not drop-in Decred implementations:

* [Poelstra, CAT and Schnorr Tricks I](https://medium.com/blockstream/cat-and-schnorr-tricks-i-faf1b59bd298): signature-based transaction introspection.
* [Linus, BitVM](https://bitvm.org/bitvm.pdf): committed computation and interactive challenges; its original construction is two-party and uses Bitcoin Taproot.
* [BitVMX](https://arxiv.org/html/2405.06842v1): dynamic message commitments and execution disputes over pre-signed templates.
* [BoLD](https://arxiv.org/abs/2404.10491): research on resisting delay attacks in multi-party execution disputes; portability to Decred is not established.

## Practical Poker exit audit

The [source-backed refusal audit](poker-exit-audit.md) distinguishes stake
settlement from the bond ladder, reproduces an owner answer that escapes the
ladder, and tests two- and six-player recovery with stock contextual maturity
checks. Run `make poker-exit-check`. These findings supersede any implication
that the existing Poker ladder guarantees StakeWars winner payouts.

The [minimal answer repair](poker-ladder-repair.md) constrains re-bonding with
pre-collected ordinary signatures. It adds no transactions and passes the node
harness, but sacrifices seed-only recovery and leaves the payout veto intact.

## Real-simulation dispute narrowing

The [action-enforcement experiment](action-enforcement-research.md) now commits
and narrows actual StakeWars simulation traces. A 512-tick prefix narrows in
nine midpoint rounds; a synthetic 20,000-tick trace takes fifteen. These are
not transaction counts and the terminal on-chain tick verifier is not built.
Run `make turntrace-check` / `make turntrace-demo`.

## Real flight arithmetic

The [flight-operation experiment](../research/flightproof/README.md) verifies the
bounded bazooka gravity/clamp arithmetic against actual simulation results.
The representative fraud transaction passes stock mempool checks at 517 bytes;
its script uses 44 operations. Canonical memory provenance, instruction selection,
state write-back and a complete dispute contract are not implemented.
