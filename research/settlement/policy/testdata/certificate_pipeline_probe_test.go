package mempool

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

// TestCertificatePipelineDepth tests only transaction dependency policy using
// ordinary signed transactions. It is not a WOTS/certificate implementation.
func TestCertificatePipelineDepth(t *testing.T) {
	h, _, err := newPoolHarness(chaincfg.MainNetParams())
	if err != nil {
		t.Fatal(err)
	}
	h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
	origin := wire.NewMsgTx()
	origin.AddTxOut(newTxOut(100_000_000, h.payScriptVer, h.payScript))
	fund := dcrutil.NewTx(origin)
	h.AddFakeUTXO(fund, 1000, 0)
	h.chain.SetHeight(1000)
	parent := fund
	size := 0
	for i := 0; i < 102; i++ {
		tx, err := h.CreateSignedTx([]spendableOutput{txOutToSpendableOut(parent, 0, wire.TxTreeRegular)}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			tx.MsgTx().TxIn[0].BlockHeight = 1000
			tx.MsgTx().TxIn[0].BlockIndex = 0
		}
		accepted, err := h.txPool.ProcessTransaction(tx, false, false, 0)
		if err != nil {
			t.Fatalf("dependency depth %d: %v", i+1, err)
		}
		if len(accepted) != 1 {
			t.Fatalf("depth %d accepted %d", i+1, len(accepted))
		}
		size += tx.MsgTx().SerializeSize()
		parent = tx
	}
	if h.chain.BestHeight() != 1000 {
		t.Fatal("unexpected chain advancement")
	}
	t.Logf("accepted 102 parent-first unconfirmed transactions with no new blocks; aggregate ordinary-tx size %d bytes; no WOTS or mining demonstrated", size)
}
