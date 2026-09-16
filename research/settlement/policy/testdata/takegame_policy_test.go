package mempool

import (
	"errors"
	"github.com/decred/dcrd/blockchain/stake/v5"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTakeGamePolicy(t *testing.T) {
	for _, tc := range []struct {
		dir, name string
		value     int64
	}{
		{"dcr-takegame-probe", "valid", 1020000},
		{"dcr-takegame-probe", "winA", 980000},
		{"dcr-takegame-probe", "winB", 1000000},
		{"dcr-takegame-probe", "timeout", 1020000},
		{"dcr-takegame-probe", "draw", 880000},
	} {
		ages := []int64{2}
		if tc.name == "timeout" || tc.name == "draw" {
			ages = []int64{1, 2}
		}
		for _, age := range ages {
			t.Run(tc.dir+"/"+tc.name+"/age"+string(rune('0'+age)), func(t *testing.T) {
				base := os.Getenv("CRYPTO_FIXTURE_DIR")
				if base == "" {
					t.Fatal("CRYPTO_FIXTURE_DIR is required")
				}
				root := filepath.Join(base, tc.dir)
				raw, e := os.ReadFile(filepath.Join(root, tc.name+".tx"))
				if e != nil {
					t.Fatal(e)
				}
				pk, e := os.ReadFile(filepath.Join(root, tc.name+"-prevout.script"))
				if e != nil && tc.name == "valid" {
					pk, e = os.ReadFile(filepath.Join(root, "prevout.script"))
				}
				if e != nil {
					t.Fatal(e)
				}
				msg := wire.NewMsgTx()
				if e = msg.FromBytes(raw); e != nil {
					t.Fatal(e)
				}
				if len(msg.TxIn) != 1 || msg.TxIn[0].ValueIn != tc.value || msg.Expiry != 0 {
					t.Fatal("fixture input or expiry differs")
				}
				h, _, e := newPoolHarness(chaincfg.MainNetParams())
				if e != nil {
					t.Fatal(e)
				}
				h.txPool.cfg.Policy.MinRelayTxFee = DefaultMinRelayTxFee
				const fundingHeight int64 = 1000
				fundingMsg := wire.NewMsgTx()
				fundingMsg.AddTxOut(wire.NewTxOut(tc.value, pk))
				funding := dcrutil.NewTx(fundingMsg)
				h.AddFakeUTXO(funding, fundingHeight, 0)
				origin := wire.OutPoint{Hash: *funding.Hash(), Index: 0, Tree: wire.TxTreeRegular}
				h.chain.utxos.Entries()[msg.TxIn[0].PreviousOutPoint] = h.chain.utxos.LookupEntry(origin)
				msg.TxIn[0].BlockHeight = uint32(fundingHeight)
				msg.TxIn[0].BlockIndex = 0
				h.chain.SetHeight(fundingHeight + age - 1)
				tx := dcrutil.NewTx(msg)
				if e = checkTransactionStandard(tx, stake.TxTypeRegular, h.chain.BestHeight()+1, time.Now(), DefaultMinRelayTxFee); e != nil {
					t.Fatal(e)
				}
				if e = checkInputsStandard(tx, stake.TxTypeRegular, h.chain.utxos, true); e != nil {
					t.Fatal(e)
				}
				var output int64
				for _, o := range msg.TxOut {
					output += o.Value
				}
				fee := tc.value - output
				minFee := calcMinRequiredTxRelayFee(int64(msg.SerializeSize()), DefaultMinRelayTxFee)
				if fee < minFee {
					t.Fatalf("fee %d < %d", fee, minFee)
				}
				accepted, e := h.txPool.ProcessTransaction(tx, false, false, 0)
				if age == 1 {
					if !errors.Is(e, ErrSeqLockUnmet) || len(accepted) != 0 {
						t.Fatalf("wanted age rejection, got %v accepted%d", e, len(accepted))
					}
				} else {
					if e != nil || len(accepted) != 1 {
						t.Fatalf("wanted acceptance, got %v accepted%d", e, len(accepted))
					}
				}
				t.Logf("stock mainnet parameters: input age%d, signature script%dB, tx%dB, paid%d required%d atoms, result %v", age, len(msg.TxIn[0].SignatureScript), msg.SerializeSize(), fee, minFee, e)
			})
		}
	}
}
