package mempool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/internal/blockchain"
	"github.com/decred/dcrd/wire"
)

// TestCovenantTimeoutInputAge uses the stock mempool's contextual checks with
// its existing fake-chain harness. Unlike an isolated txscript evaluation,
// this checks the confirmed input age against the candidate next block.
// The funding UTXO is synthetic; this is not block generation or propagation.
func TestCovenantTimeoutInputAge(t *testing.T) {
	root := os.Getenv("CRYPTO_FIXTURE_DIR")
	if root == "" {
		root = "/tmp"
	}
	raw, err := os.ReadFile(filepath.Join(root, "dcr-recursive-probe", "timeout.tx"))
	if err != nil {
		t.Fatal(err)
	}
	pk, err := os.ReadFile(filepath.Join(root, "dcr-recursive-probe", "timeout-prevout.script"))
	if err != nil {
		t.Fatal(err)
	}
	for _, age := range []int64{1, 2} {
		name := "age1_reject"
		if age == 2 {
			name = "age2_accept"
		}
		t.Run(name, func(t *testing.T) {
			h, _, err := newPoolHarness(chaincfg.MainNetParams())
			if err != nil {
				t.Fatal(err)
			}
			h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
			msg := wire.NewMsgTx()
			if err = msg.FromBytes(raw); err != nil {
				t.Fatal(err)
			}
			if len(msg.TxIn) != 1 || msg.TxIn[0].Sequence != 2 || msg.Expiry != 0 {
				t.Fatal("fixture must have one input, block CSV=2, and no expiry")
			}
			const originHeight int64 = 1000
			origin := wire.NewMsgTx()
			origin.AddTxOut(wire.NewTxOut(msg.TxIn[0].ValueIn, pk))
			funding := dcrutil.NewTx(origin)
			h.AddFakeUTXO(funding, originHeight, 0)
			original := wire.OutPoint{Hash: *funding.Hash(), Index: 0, Tree: wire.TxTreeRegular}
			// Alias the synthetic UTXO to the already-signed fixture outpoint.
			h.chain.utxos.Entries()[msg.TxIn[0].PreviousOutPoint] = h.chain.utxos.LookupEntry(original)
			msg.TxIn[0].BlockHeight = uint32(originHeight)
			msg.TxIn[0].BlockIndex = 0
			h.chain.SetHeight(originHeight + age - 1)
			accepted, err := h.txPool.ProcessTransaction(dcrutil.NewTx(msg), false, false, 0)
			if age == 1 {
				if !errors.Is(err, ErrSeqLockUnmet) {
					t.Fatalf("age1: got %v, want ErrSeqLockUnmet", err)
				}
				if len(accepted) != 0 {
					t.Fatalf("age1 unexpectedly accepted %d transactions", len(accepted))
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(accepted) != 1 {
					t.Fatalf("age2 accepted %d transactions", len(accepted))
				}
			}
			t.Logf("input height %d, candidate block %d, age %d: %v", originHeight, h.chain.BestHeight()+1, age, err)
		})
	}
}

// TestExpiryOutputMaturityMainnetContext isolates the mainnet-specific
// maturity consequence of a parent carrying Expiry. The child itself has no
// expiry and no relative timelock; only the parent's HasExpiry flag changes.
func TestExpiryOutputMaturityMainnetContext(t *testing.T) {
	tests := []struct {
		name      string
		hasExpiry bool
		age       int64
		want      error
	}{
		{"no_expiry_age1", false, 1, nil},
		{"expiry_age255", true, 255, blockchain.ErrExpiryTxSpentEarly},
		{"expiry_age256", true, 256, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := chaincfg.MainNetParams()
			if params.CoinbaseMaturity != 256 {
				t.Fatalf("mainnet maturity changed: %d", params.CoinbaseMaturity)
			}
			h, _, err := newPoolHarness(params)
			if err != nil {
				t.Fatal(err)
			}
			h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
			const originHeight int64 = 1000
			origin := wire.NewMsgTx()
			if tt.hasExpiry {
				origin.Expiry = uint32(originHeight + 10)
			}
			origin.AddTxOut(newTxOut(1_000_000, h.payScriptVer, h.payScript))
			funding := dcrutil.NewTx(origin)
			h.AddFakeUTXO(funding, originHeight, 0)
			child, err := h.CreateSignedTx([]spendableOutput{txOutToSpendableOut(funding, 0, wire.TxTreeRegular)}, 1)
			if err != nil {
				t.Fatal(err)
			}
			child.MsgTx().TxIn[0].BlockHeight = uint32(originHeight)
			child.MsgTx().TxIn[0].BlockIndex = 0
			h.chain.SetHeight(originHeight + tt.age - 1)
			accepted, err := h.txPool.ProcessTransaction(child, false, false, 0)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if tt.want == nil && len(accepted) != 1 {
				t.Fatalf("accepted %d, want 1", len(accepted))
			}
			if tt.want != nil && len(accepted) != 0 {
				t.Fatalf("accepted %d despite rejection", len(accepted))
			}
			t.Logf("parent expiry=%d, child expiry=%d, input age=%d: %v", origin.Expiry, child.MsgTx().Expiry, tt.age, err)
		})
	}
}
