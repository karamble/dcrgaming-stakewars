package mempool

import (
	"bytes"
	"errors"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func loadWalletCheckpointFixture(t *testing.T, name string) (*wire.MsgTx, []byte) {
	t.Helper()
	root := filepath.Join(os.Getenv("CRYPTO_FIXTURE_DIR"), "dcr-checkpoint-channel-probe")
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
func walletCheckpointPool(t *testing.T, tx *wire.MsgTx, pk []byte, value, age int64) *poolHarness {
	t.Helper()
	h, _, e := newPoolHarness(chaincfg.MainNetParams())
	if e != nil {
		t.Fatal(e)
	}
	h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
	fund := wire.NewMsgTx()
	fund.AddTxOut(wire.NewTxOut(value, pk))
	funding := dcrutil.NewTx(fund)
	h.AddFakeUTXO(funding, 1000, 0)
	op := wire.OutPoint{Hash: *funding.Hash(), Index: 0, Tree: wire.TxTreeRegular}
	h.chain.utxos.Entries()[tx.TxIn[0].PreviousOutPoint] = h.chain.utxos.LookupEntry(op)
	tx.TxIn[0].BlockHeight = 1000
	tx.TxIn[0].BlockIndex = 0
	h.chain.SetHeight(1000 + age - 1)
	return h
}

func TestWalletCheckpointChannelStockPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		value, age int64
		want       error
	}{
		{"claim", 1060000, 1000, nil},
		{"game-final", 980000, 1, nil},
		{"stale-alternative", 1000000, 1, nil},
		{"update", 1020000, 1, nil},
		{"stale-exit", 1020000, 1, ErrSeqLockUnmet},
		{"stale-exit", 1020000, 2, nil},
		{"latest-exit", 1000000, 1, ErrSeqLockUnmet},
		{"latest-exit", 1000000, 2, nil},
	} {
		t.Run(tc.name+"/age"+strconv.FormatInt(tc.age, 10), func(t *testing.T) {
			tx, pk := loadWalletCheckpointFixture(t, tc.name)
			if tx.TxIn[0].ValueIn != tc.value {
				t.Fatal("unexpected funding value")
			}
			h := walletCheckpointPool(t, tx, pk, tc.value, tc.age)
			got, e := h.txPool.ProcessTransaction(dcrutil.NewTx(tx), false, false, 0)
			if !errors.Is(e, tc.want) {
				t.Fatalf("got%v want%v", e, tc.want)
			}
			if (tc.want == nil) != (len(got) == 1) {
				t.Fatalf("accepted %d", len(got))
			}
			t.Logf("age%d sigscript%d tx%d result%v", tc.age, len(tx.TxIn[0].SignatureScript), tx.SerializeSize(), e)
		})
	}
}

func TestWalletCheckpointChannelFirstSeenExitRace(t *testing.T) {
	for _, first := range []string{"update", "stale-exit"} {
		t.Run(first+"_first", func(t *testing.T) {
			firstTx, pk := loadWalletCheckpointFixture(t, first)
			second := "update"
			if first == "update" {
				second = "stale-exit"
			}
			secondTx, _ := loadWalletCheckpointFixture(t, second)
			if firstTx.TxIn[0].PreviousOutPoint != secondTx.TxIn[0].PreviousOutPoint {
				t.Fatal("fixtures don't conflict")
			}
			h := walletCheckpointPool(t, firstTx, pk, 1020000, 2)
			secondTx.TxIn[0].BlockHeight = 1000
			secondTx.TxIn[0].BlockIndex = 0
			a, e := h.txPool.ProcessTransaction(dcrutil.NewTx(firstTx), false, false, 0)
			if e != nil || len(a) != 1 {
				t.Fatalf("first %v count%d", e, len(a))
			}
			a, e = h.txPool.ProcessTransaction(dcrutil.NewTx(secondTx), false, false, 0)
			if !errors.Is(e, ErrMempoolDoubleSpend) || len(a) != 0 {
				t.Fatalf("second %v count%d", e, len(a))
			}
			t.Logf("%s accepted; conflicting %s rejected despite valid checkpoint signatures: %v", first, second, e)
		})
	}
}

func TestWalletCheckpointLinkedFixtureAmounts(t *testing.T) {
	for _, pair := range [][2]string{{"claim", "update"}, {"claim", "stale-exit"}, {"update", "latest-exit"}, {"latest-exit", "game-final"}, {"stale-exit", "stale-alternative"}} {
		parent, _ := loadWalletCheckpointFixture(t, pair[0])
		child, pk := loadWalletCheckpointFixture(t, pair[1])
		if child.TxIn[0].PreviousOutPoint.Hash != parent.TxHash() || child.TxIn[0].PreviousOutPoint.Index != 0 {
			t.Fatalf("%s does not spend %s", pair[1], pair[0])
		}
		if !bytes.Equal(pk, parent.TxOut[0].PkScript) || child.TxIn[0].ValueIn != parent.TxOut[0].Value {
			t.Fatalf("%s wrong funding script or amount", pair[1])
		}
	}
}
