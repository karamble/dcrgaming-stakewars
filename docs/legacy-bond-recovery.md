# Legacy admission bond recovery

One mainnet admission bond predates the bridge-controlled key scheme. It is
recovered manually, with a key held outside this repository.

This note is **non-secret** and must stay that way. The seed and private key
never go in this repository, chat, logs, or an assistant memory file.

## The deposit

- **0.01 DCR** (1,000,000 atoms), per-table admission bond.
- The configured stake of 0.001 DCR was never funded, so there is nothing else
  at this table to recover. The second seat's approval expired without payment
  and has no refundable output at all.
- Table: `641be69ec6100361ff3e35357355c9d1`
- Outpoint: `cdaf64d5cb5e1e8eb382366e753596bd12acda9a63de302ce2f525a5d8497b31:0`
- Address: `DcY9LDHrPTK89NZuZetYvDr5fToQDrM61pU`
- Funding block: `1115551`,
  hash `882c9b301a5659fa83c226c70116a06276e4c44c2dbb02d58661df3d49bef9f9`
- Refund lock: **2016 blocks**, not the stake's 288.
- Earliest refund inclusion height: **1117567**, while the funding stays at the
  recorded height. Recheck against the chain, including reorgs; a height is the
  only reliable form of this answer.

## Scripts and derivation

The private key is a raw 32-byte secp256k1 scalar encoded as hexadecimal, not
WIF.

- Derivation: `HMAC-BLAKE256(seed, UTF8("StakeWars/bond/v1") || UTF8(table ID))`
- Public key: `03403f52fa959ae8d0122893a0e28df9946421ada139dd0be07291edd4fdf6731a`
- Redeem script:
  `02e007b2752103403f52fa959ae8d0122893a0e28df9946421ada139dd0be07291edd4fdf6731a52bf51`
- P2SH output script: `a91407843eb0deee852bb3eb51998fa36f501cb6e59687`

The backup holds the original `identity.json`, `table.json` and `spends.json`,
a derived `recovery-private-key.hex`, a public `recovery.json` and a `README.txt`.
Its directory is `0700` and every file `0600`. The seed and derived key were
verified against the funding P2SH script, and possession was proven with a
non-transaction Schnorr challenge signature. No refund has been signed or
broadcast.

## Recovering it

1. Check the output against a node: network, amount, script, confirmations,
   unspent state and any pending spend. An explorer result alone does not
   establish that the output is still unspent.
2. Sign with a separate manual tool, using the backed-up key and the redeem
   script above. Do not add a legacy import or compatibility path to the bridge
   or the SDK — keys the bridge controls do not control this output.
3. Show the destination, fee and returned amount for approval before signing and
   broadcasting. No counterparty has to cooperate. Keep the original profile and
   the backup until the spend confirms.
