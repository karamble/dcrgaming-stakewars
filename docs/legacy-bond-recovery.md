# Local Omarchy legacy bond recovery

Verified and backed up on 2026-09-16. This is a persistent **non-secret** recovery
note. Never put the seed or private key in this repository, chat, logs, or an
assistant memory file.

- Instance: local Omarchy dcrpulse; StakeWars **player2**.
- Original profile: `/home/user/.dcrstakewars-player2`.
- Backup: `/home/user/.local/share/dcrstakewars/recovery/omarchy-player2-641be69ec6100361ff3e35357355c9d1`.
- Backup directory permission: `0700`; every file: `0600`.
- Backup contains the original `identity.json`, `table.json`, `spends.json`, a
  derived `recovery-private-key.hex`, public `recovery.json`, and `README.txt`.
- The seed and derived key were verified against the actual funding P2SH script;
  a non-transaction Schnorr challenge signature verified possession. Backup
  copies were compared byte-for-byte with the originals. No refund was signed
  or broadcast.

## Payment

- **0.01 DCR** (1,000,000 atoms), per-table admission bond.
- **0.001 DCR** was the configured stake; that stake was **never funded**.
- JACKIN/player1's approval expired without payment; it has no refundable output
  for this attempt. Do not confuse the profiles.
- Table: `641be69ec6100361ff3e35357355c9d1`.
- Outpoint: `cdaf64d5cb5e1e8eb382366e753596bd12acda9a63de302ce2f525a5d8497b31:0`.
- Mainnet address: `DcY9LDHrPTK89NZuZetYvDr5fToQDrM61pU`.
- Funding block: `1115551`, hash
  `882c9b301a5659fa83c226c70116a06276e4c44c2dbb02d58661df3d49bef9f9`.
- Refund lock: **2016 blocks**, not the stake's 288-block lock.
- Earliest refund inclusion height: **1117567**, if the funding remains at the
  recorded height; eligible for the next block at tip **1117566**.
- Rough calendar estimate: **23 September 2026** at target block times. Actual
  maturity depends on chain progress and must be rechecked, including reorgs.

## Recovering later

1. Read the public `recovery.json` and check the output against the local Omarchy
   node: current network, original amount/script, confirmations, unspent state,
   and any pending spend. A transaction explorer result alone does not establish
   that the output remains unspent.
2. Use a separate manual recovery tool with the backed-up key and original
   redeem script. Do not add a legacy import or compatibility path to the new
   bridge/SDK. New bridge-controlled keys do not control this existing output.
3. Show the local wallet destination, transaction fee, and returned amount for
   operator approval before signing/broadcast. Recovery does not require JACKIN
   to cooperate. Preserve the original profile and backup until confirmed.

The private key is a raw 32-byte secp256k1 scalar encoded as hexadecimal, not WIF.
Derivation: HMAC-BLAKE256(seed, UTF8(`StakeWars/bond/v1`) || UTF8(table ID)).
Public key: `03403f52fa959ae8d0122893a0e28df9946421ada139dd0be07291edd4fdf6731a`.
Redeem script:
`02e007b2752103403f52fa959ae8d0122893a0e28df9946421ada139dd0be07291edd4fdf6731a52bf51`.
P2SH output script: `a91407843eb0deee852bb3eb51998fa36f501cb6e59687`.
