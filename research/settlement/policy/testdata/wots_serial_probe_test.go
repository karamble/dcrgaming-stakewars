package mempool

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	tscript "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestWOTSSerialStockPolicy(t *testing.T) {
	root := filepath.Join(os.Getenv("CRYPTO_FIXTURE_DIR"), "dcr-wots-serial")
	load := func(name string) (*wire.MsgTx, []byte) {
		raw, e := os.ReadFile(filepath.Join(root, name+".tx"))
		if e != nil {
			t.Fatal(e)
		}
		pk, e := os.ReadFile(filepath.Join(root, name+"-prevout.script"))
		if e != nil {
			t.Fatal(e)
		}
		tx := wire.NewMsgTx()
		if e = tx.FromBytes(raw); e != nil {
			t.Fatal(e)
		}
		return tx, pk
	}
	first, pk := load("valid")
	child, _ := load("linked")
	if first.TxIn[0].ValueIn != 3000000 || first.TxOut[0].Value != 2980000 || child.TxIn[0].PreviousOutPoint.Hash != first.TxHash() || child.TxIn[0].ValueIn != 2980000 || child.TxOut[0].Value != 2960000 {
		t.Fatal("unexpected linked pot values")
	}
	h, _, e := newPoolHarness(chaincfg.MainNetParams())
	if e != nil {
		t.Fatal(e)
	}
	h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
	// Match mainnet after the activated lnfeatures agenda. The fake-chain
	// harness defaults to base flags and does not calculate agenda state.
	h.chain.SetStandardVerifyFlags(BaseStandardVerifyFlags | tscript.ScriptVerifySHA256)
	fundingMsg := wire.NewMsgTx()
	fundingMsg.AddTxOut(wire.NewTxOut(3000000, pk))
	funding := dcrutil.NewTx(fundingMsg)
	h.AddFakeUTXO(funding, 499999, 0)
	origin := wire.OutPoint{Hash: *funding.Hash(), Index: 0, Tree: wire.TxTreeRegular}
	h.chain.utxos.Entries()[first.TxIn[0].PreviousOutPoint] = h.chain.utxos.LookupEntry(origin)
	h.chain.SetHeight(500000)
	first.TxIn[0].BlockHeight = 499999
	first.TxIn[0].BlockIndex = 0
	for i, tx := range []*wire.MsgTx{first, child} {
		accepted, e := h.txPool.ProcessTransaction(dcrutil.NewTx(tx), false, false, 0)
		if e != nil || len(accepted) != 1 {
			t.Fatalf("fragment%d: %v accepted%d", i, e, len(accepted))
		}
		t.Logf("linked RFC8391 fragment%d accepted: signature script%dB tx%dB fee20000 minfee%d", i, len(tx.TxIn[0].SignatureScript), tx.SerializeSize(), calcMinRequiredTxRelayFee(int64(tx.SerializeSize()), DefaultMinRelayTxFee))
	}
}
