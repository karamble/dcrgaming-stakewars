# One fixed instruction fraud proof

Offline fixture only; all keys and funding inputs are synthetic.

A complete memory of 524,288 one-byte cells (values 1–100) is committed with
BLAKE256. The script fixes address 21,917, the memory root, and a claimed result
for `nextRegister = 7 + memory[address]`. A 19-sibling Merkle proof authenticates
the cell; only an incorrect result permits the fixed beneficiary output.
Leaves hash `0x00 || value`; internal nodes hash two exactly 32-byte children.
The fixed-output CAT/Schnorr gadget binds the proof to the spending transaction.

The fixture cell is 18: the correct result 25 cannot be challenged, while the
false claim 108 can. Tests reject all 19 individually corrupted siblings,
malformed sibling lengths, wrong cells, forged transaction witnesses and altered
outputs even with rebuilt transaction witnesses.

Measured redeem script: 425 bytes; signature script: 1,183 bytes. The fixture
funds 30,000 atoms and pays 10,000 atoms, leaving 20,000 for fees. The policy
suite checks stock mempool admission with a synthetic UTXO.

This is only a fraud branch for one fixed instruction. It does not authenticate
a game program, establish which trace/state is authoritative, offer an honest
claim timeout, update memory, handle multiple challengers, or provide data
availability. An escrow dispute protocol must establish those properties before
this primitive could decide any real payout. Never fund this fixture.
