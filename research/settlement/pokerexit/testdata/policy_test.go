package mempool

import (
	"errors"
	"fmt"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestStakeWarsPokerExitPolicy(t *testing.T) {
	for _, n := range []int{2, 6} {
		for _, c := range []struct {
			name   string
			age    int64
			value  int64
			accept bool
		}{
			{"refund", 63, 100000000, false}, {"refund", 64, 100000000, true},
			{"take", 2, 990000, false}, {"take", 3, 990000, true},
			{"redirect", 1, 990000, true},
			{"guarded-answer", 1, 990000, true},
			{"guarded-take", 2, 990000, false}, {"guarded-take", 3, 990000, true},
		} {
			t.Run(fmt.Sprintf("%s-%d-age%d", c.name, n, c.age), func(t *testing.T) {
				base := filepath.Join(os.Getenv("POKER_EXIT_FIXTURES"), fmt.Sprintf("%s-%d", c.name, n))
				raw, err := os.ReadFile(base + ".tx")
				if err != nil {
					t.Fatal(err)
				}
				pk, err := os.ReadFile(base + ".script")
				if err != nil {
					t.Fatal(err)
				}
				tx := wire.NewMsgTx()
				if err = tx.FromBytes(raw); err != nil {
					t.Fatal(err)
				}
				if len(tx.TxIn) != 1 || tx.TxIn[0].ValueIn != c.value {
					t.Fatal("unexpected funding")
				}
				h, _, err := newPoolHarness(chaincfg.MainNetParams())
				if err != nil {
					t.Fatal(err)
				}
				h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
				origin := wire.NewMsgTx()
				origin.AddTxOut(wire.NewTxOut(c.value, pk))
				fund := dcrutil.NewTx(origin)
				h.AddFakeUTXO(fund, 1000, 0)
				op := wire.OutPoint{Hash: *fund.Hash()}
				h.chain.utxos.Entries()[tx.TxIn[0].PreviousOutPoint] = h.chain.utxos.LookupEntry(op)
				tx.TxIn[0].BlockHeight = 1000
				tx.TxIn[0].BlockIndex = 0
				h.chain.SetHeight(1000 + c.age - 1)
				accepted, err := h.txPool.ProcessTransaction(dcrutil.NewTx(tx), false, false, 0)
				if c.accept {
					if err != nil || len(accepted) != 1 {
						t.Fatalf("want admission: %v", err)
					}
				} else if !errors.Is(err, ErrSeqLockUnmet) {
					t.Fatalf("want immature rejection: %v", err)
				}
				t.Logf("input age%d, admitted%v, size%d: %v", c.age, len(accepted) == 1, tx.SerializeSize(), err)
			})
		}
	}
}
