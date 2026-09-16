package mempool

import (
	"github.com/decred/dcrd/blockchain/stake/v5"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/internal/blockchain"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCovenantProbePolicy(t *testing.T) {
	root := os.Getenv("CRYPTO_FIXTURE_DIR")
	if root == "" {
		root = "/tmp"
	}
	for _, name := range []string{"dcr-covenant-probe", "dcr-recursive-probe", "dcr-memory-proof", "dcr-flight-proof"} {
		t.Run(name, func(t *testing.T) {
			b, e := os.ReadFile(filepath.Join(root, name, "valid.tx"))
			if e != nil {
				t.Fatal(e)
			}
			pk, e := os.ReadFile(filepath.Join(root, name, "prevout.script"))
			if e != nil {
				t.Fatal(e)
			}
			msg := wire.NewMsgTx()
			if e = msg.FromBytes(b); e != nil {
				t.Fatal(e)
			}
			tx := dcrutil.NewTx(msg)
			if e = checkTransactionStandard(tx, stake.TxTypeRegular, 1000000, time.Now(), DefaultMinRelayTxFee); e != nil {
				t.Fatal(e)
			}
			origin := wire.NewMsgTx()
			origin.AddTxOut(wire.NewTxOut(msg.TxIn[0].ValueIn, pk))
			fund := dcrutil.NewTx(origin)
			view := blockchain.NewUtxoViewpoint(nil)
			view.AddTxOut(fund, 0, 1, 0, true)
			key := wire.OutPoint{Hash: *fund.Hash(), Index: 0, Tree: wire.TxTreeRegular}
			view.Entries()[msg.TxIn[0].PreviousOutPoint] = view.LookupEntry(key)
			if e = checkInputsStandard(tx, stake.TxTypeRegular, view, true); e != nil {
				t.Fatal(e)
			}
			var outValue int64
			for _, o := range msg.TxOut {
				outValue += o.Value
			}
			fee := msg.TxIn[0].ValueIn - outValue
			minFee := calcMinRequiredTxRelayFee(int64(msg.SerializeSize()), DefaultMinRelayTxFee)
			if fee < minFee {
				t.Fatalf("fee %d < required %d", fee, minFee)
			}
			harness, _, e := newPoolHarness(chaincfg.MainNetParams())
			if e != nil {
				t.Fatal(e)
			}
			harness.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
			harness.chain.utxos.Entries()[msg.TxIn[0].PreviousOutPoint] = view.LookupEntry(key)
			msg.TxIn[0].BlockHeight = 1
			msg.TxIn[0].BlockIndex = 0
			accepted, e := harness.txPool.ProcessTransaction(dcrutil.NewTx(msg), false, false, 0)
			if e != nil {
				t.Fatal(e)
			}
			if len(accepted) != 1 {
				t.Fatalf("accepted %d", len(accepted))
			}
			t.Logf("ProcessTransaction accepted with mainnet min relay fee %d; paid %d atoms", DefaultMinRelayTxFee, fee)
			t.Logf("transaction and input standardness pass; size=%d required relay fee=%d atoms; synthetic UTXO; mock-chain mempool acceptance, not live-network propagation", msg.SerializeSize(), calcMinRequiredTxRelayFee(int64(msg.SerializeSize()), DefaultMinRelayTxFee))
		})
	}
}
