# WOTS+ verification fragments

Research only. This package measures the real SHA2-256 WOTS+ construction from
[RFC 8391](https://www.rfc-editor.org/rfc/rfc8391.html), using n=32 and w=16.
It does not authorize real checkpoints or move funds.

## What runs

`signature.go` implements the reference message-to-digit conversion, checksum,
key derivation from supplied secrets, signing, and verification. There are 64
message chains and three checksum chains. Each signature is 2,144 bytes.

`step.go` emits Decred script for one actual RFC chain step: the two addressed PRF
calls, the full 32-byte mask XOR, and the keyed F function. It does not substitute
unmasked repeated hashing. Script has only signed integer XOR, so the emitter
uses three-byte chunks with a removable positive-number sentinel.

The constant-parameter emitter takes X and produces Y. The dynamic emitter takes
X, public SEED and ADRS, checks their lengths, and produces Y. Its caller must
authenticate the seed, address, public-key commitment, position and message.

## Measurements

| Fragment | Script bytes | Non-push operations |
|---|---:|---:|
| Fixed seed/address, one step | 427 | 126 |
| Dynamic seed/address, one step | 391 | 141 |

Adding the 32-byte expected result and equality check adds 34 bytes and one
operation. These figures exclude a covenant wrapper, authenticated public-key
proof, chain initialization/finalization, and transaction witness overhead.
They must not be presented as complete signature-verifier costs.

Tests execute every needed step for one arbitrary digest through Decred's script
engine: the fixture requires 435 steps across all 67 chains. Another 100 vectors
exercise each emitter. Modified message, checksum-chain signature, public seed
and OTS address fail reference signature verification.

The exact chain-step range over 32-byte messages is 45–990. The all-zero digest
needs 960 message steps and 30 checksum steps; the all-ones digest needs 45 checksum
steps. A design using one step per transaction would therefore need up to 990
transactions per signer, or 5,940 for six signers, before auxiliary protocol
transitions. This is a cost warning, not an implemented end-to-end protocol.

## Limits

Each secret key must sign at most one message. These fixtures deliberately use
deterministic public test secrets; there is no production key generation,
persistent key-slot allocation, XMSS Merkle tree or atomic checkpoint exchange.

Individual successful step checks do not establish whole-signature authorization.
A surrounding protocol must bind every fragment to the same immutable message,
enforce all 67 chains and checksum, authenticate each endpoint under the enrolled
signer key, prevent skipped/duplicated fragments, and only release the pot after
all required signers pass. Interruption, timeout and failed-claim recovery are
also separate requirements.

Run from the project root:

```sh
go -C research/settlement test -count=1 -v ./wots
```
