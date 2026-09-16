package protocolcheck

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
	"github.com/karamble/dcrgaming-sdk/pkg/finance"
	"github.com/karamble/dcrstakewars/internal/payout"
)

const (
	fixtureStake = int64(1_000_000)
	fixtureFee   = int64(20_000)
	fixtureLock  = uint32(288)
)

// Private financial keys exist only in this script-engine fixture, representing
// independent wallets. The production game only constructs public proposals.
func settlement(t *testing.T, n int, shares []int64) (*wire.MsgTx, []finance.Input, []*secp256k1.PrivateKey, int64) {
	t.Helper()
	keys := make([]*secp256k1.PrivateKey, n)
	for i := range keys {
		raw := make([]byte, 32)
		raw[31] = byte(i + 1)
		keys[i] = secp256k1.PrivKeyFromBytes(raw)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i].PubKey().SerializeCompressed(), keys[j].PubKey().SerializeCompressed()) < 0
	})
	pubs := make([]string, n)
	destinations := map[string]string{}
	for i, key := range keys {
		pub := key.PubKey().SerializeCompressed()
		pubs[i] = hex.EncodeToString(pub)
		address, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(stdaddr.Hash160(pub), chaincfg.SimNetParams())
		if err != nil {
			t.Fatal(err)
		}
		destinations[pubs[i]] = address.String()
	}
	proposal := finance.Payout{Table: "abcdef01"}
	for i, pub := range pubs {
		// Membership identity is distinct from the key which controls the deposit.
		identity := make([]byte, 32)
		identity[31] = byte(i + 100)
		var h chainhash.Hash
		h[0] = byte(i + 1)
		terms := finance.Terms{Version: finance.Version, Game: "stakewars", Network: "simnet", Table: proposal.Table, Kind: "stake", Atoms: fixtureStake, LockBlocks: fixtureLock, Identity: hex.EncodeToString(secp256k1.PrivKeyFromBytes(identity).PubKey().SerializeCompressed()), Recovery: pub, Members: pubs}
		proposal.Inputs = append(proposal.Inputs, finance.Input{Terms: terms, Outpoint: wire.OutPoint{Hash: h}})
		if shares[i] > 0 {
			proposal.Payments = append(proposal.Payments, finance.Payment{Key: pub, Atoms: shares[i]})
		}
	}
	built, err := finance.BuildPayout(proposal, destinations, chaincfg.SimNetParams())
	if err != nil {
		t.Fatal(err)
	}
	return built.Transaction, built.Inputs, keys, built.FeeAtoms
}

func signatures(t *testing.T, tx *wire.MsgTx, inputs []finance.Input, keys []*secp256k1.PrivateKey) []map[string][]byte {
	t.Helper()
	result := make([]map[string][]byte, len(inputs))
	for i, in := range inputs {
		hash, err := finance.SignatureHash(tx, i, in)
		if err != nil {
			t.Fatal(err)
		}
		result[i] = map[string][]byte{}
		for _, key := range keys {
			result[i][hex.EncodeToString(key.PubKey().SerializeCompressed())] = ecdsa.Sign(key, hash).Serialize()
		}
	}
	return result
}
func finish(tx *wire.MsgTx, inputs []finance.Input, sigs []map[string][]byte) (*wire.MsgTx, error) {
	signed := tx.Copy()
	for i, in := range inputs {
		witness, err := finance.SettlementWitness(signed, i, in, sigs[i])
		if err != nil {
			return nil, err
		}
		signed.TxIn[i].SignatureScript = witness
	}
	if err := finance.VerifySpend(signed, inputs, chaincfg.SimNetParams()); err != nil {
		return nil, err
	}
	return signed, nil
}
func winnerShares(n int) []int64 {
	shares := make([]int64, n)
	shares[0] = int64(n) * fixtureStake
	return shares
}

func TestBridgeSettlementWithAllSignaturesPaysWinner(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tx, inputs, keys, fee := settlement(t, n, winnerShares(n))
			paid, err := finish(tx, inputs, signatures(t, tx, inputs, keys))
			if err != nil {
				t.Fatal(err)
			}
			if len(paid.TxOut) != 1 || paid.TxOut[0].Value != int64(n)*fixtureStake-fee {
				t.Fatal("incorrect winner payment")
			}
		})
	}
}
func TestCounterexampleEliminatingAbsentSeatDoesNotAuthorizeItsStake(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tx, inputs, keys, _ := settlement(t, n, winnerShares(n))
			partial := signatures(t, tx, inputs, keys[:n-1])
			if _, err := finish(tx, inputs, partial); err == nil {
				t.Fatal("missing loser signature accepted")
			}
			missing := hex.EncodeToString(keys[n-1].PubKey().SerializeCompressed())
			survivor := hex.EncodeToString(keys[0].PubKey().SerializeCompressed())
			for _, sigs := range partial {
				sigs[missing] = sigs[survivor]
			}
			if _, err := finish(tx, inputs, partial); err == nil {
				t.Fatal("survivor substituted for missing loser")
			}
		})
	}
}
func TestBridgeCustomSharesConservePotAndRejectChangedPayout(t *testing.T) {
	policy, err := payout.WinnerTakesAll(3, fixtureStake)
	if err != nil {
		t.Fatal(err)
	}
	policy.Gross[0] = []int64{1800000, 900000, 300000}
	shares, err := policy.Allocation(0)
	if err != nil {
		t.Fatal(err)
	}
	tx, inputs, keys, fee := settlement(t, 3, shares)
	sigs := signatures(t, tx, inputs, keys)
	paid, err := finish(tx, inputs, sigs)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]int64(nil), shares...)
	share, remainder := fee/int64(len(want)), fee%int64(len(want))
	for i := range want {
		want[i] -= share
		if i == 0 {
			want[i] -= remainder
		}
	}
	for i, out := range paid.TxOut {
		if out.Value != want[i] {
			t.Fatal("incorrect fee distribution")
		}
	}
	changed := tx.Copy()
	changed.TxOut[0].Value--
	changed.TxOut[1].Value++
	if _, err = finish(changed, inputs, sigs); err == nil {
		t.Fatal("changed allocation accepted")
	}
	changed = tx.Copy()
	changed.TxOut[0].PkScript = append([]byte(nil), tx.TxOut[1].PkScript...)
	if _, err = finish(changed, inputs, sigs); err == nil {
		t.Fatal("changed destination accepted")
	}
}
func TestBridgeRefundRequiresOwnerAndSufficientSequence(t *testing.T) {
	for n := 2; n <= 6; n++ {
		_, inputs, keys, _ := settlement(t, n, winnerShares(n))
		for owner, in := range inputs {
			t.Run(fmt.Sprintf("%d/owner%d", n, owner), func(t *testing.T) {
				pub := keys[owner].PubKey().SerializeCompressed()
				a, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(stdaddr.Hash160(pub), chaincfg.SimNetParams())
				if err != nil {
					t.Fatal(err)
				}
				_, pay := a.PaymentScript()
				refund, err := finance.Refund(in, pay, fixtureFee)
				if err != nil {
					t.Fatal(err)
				}
				sign := func(tx *wire.MsgTx, key *secp256k1.PrivateKey) error {
					hash, err := finance.SignatureHash(tx, 0, in)
					if err != nil {
						return err
					}
					witness, err := finance.RefundWitness(tx, 0, in, ecdsa.Sign(key, hash).Serialize())
					if err != nil {
						return err
					}
					tx.TxIn[0].SignatureScript = witness
					return finance.VerifySpend(tx, []finance.Input{in}, chaincfg.SimNetParams())
				}
				if err = sign(refund, keys[owner]); err != nil {
					t.Fatal(err)
				}
				if refund.TxOut[0].Value != fixtureStake-fixtureFee || !bytes.Equal(refund.TxOut[0].PkScript, pay) {
					t.Fatal("incorrect owner refund")
				}
				early := refund.Copy()
				early.TxIn[0].Sequence--
				if err = sign(early, keys[owner]); err == nil {
					t.Fatal("insufficient sequence accepted")
				}
				if err = sign(refund.Copy(), keys[(owner+1)%n]); err == nil {
					t.Fatal("foreign refund key accepted")
				}
			})
		}
	}
	// Actual UTXO age and wallet operation are separate bridge integration checks.
}
func TestCollectedSettlementSignaturesSurviveDisconnect(t *testing.T) {
	for n := 2; n <= 6; n++ {
		tx, inputs, keys, _ := settlement(t, n, winnerShares(n))
		collected := signatures(t, tx, inputs, keys)
		keys = nil
		if _, err := finish(tx, inputs, collected); err != nil {
			t.Fatal(err)
		}
		changed := tx.Copy()
		changed.TxOut[0].Value--
		if _, err := finish(changed, inputs, collected); err == nil {
			t.Fatal("presigned payment changed")
		}
	}
}
