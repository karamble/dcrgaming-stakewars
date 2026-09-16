// Injected into the actual dcrpoker escrow package by check.sh. No copied scripts.
package escrow

import (
	"bytes"
	"fmt"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
	"testing"
)

const auditFee int64 = 20000
const auditBondFee int64 = 10000

func TestStakeWarsPokerRefusal(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			amounts := make([]int64, n)
			amounts[0] = int64(n) * 100000000
			s := settleTable(t, 100000000, amounts)
			s.draft.FeeAtoms = auditFee
			tx, err := BuildSettlement(s.draft)
			if err != nil {
				t.Fatal(err)
			}
			// Setup control: a fully signed result is one broadcast, with no new signer.
			all := s.sign(t, tx)
			complete, err := FinishSettlement(tx, s.draft, all, chaincfg.MainNetParams())
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%d seats: fully signed winner payout = 1 tx, %d bytes, fee %d, winner %d atoms", n, complete.SerializeSize(), auditFee, complete.TxOut[0].Value)
			// Bob withholds his settlement signatures. Forged correctly sized signatures
			// exercise consensus rather than only the API's missing-signature count check.
			for i := range all {
				all[i][n-1] = make([]byte, SigLen)
			}
			if _, err = FinishSettlement(tx, s.draft, all, chaincfg.MainNetParams()); err == nil {
				t.Fatal("winner paid without Bob")
			}
			// Alice only has her own signing key during recovery.
			own := s.draft.Inputs[0]
			refund, err := BuildTimelockedSpend(Spend{Key: privFor(t, s.privs, s.members[0]), Script: own.Redeem, Prevout: own.Prevout, ValueAtoms: own.ValueAtoms, CSVBlocks: testCSVBlocks, PayScript: s.draft.Pays[0], FeeAtoms: auditFee, SigScript: RefundSigScript, Params: chaincfg.MainNetParams()})
			if err != nil {
				t.Fatal(err)
			}
			if refund.TxOut[0].Value != 99980000 {
				t.Fatal("refund is not original stake less fee")
			}
			auditExport(t, fmt.Sprintf("refund-%d", n), refund, own.Redeem)
			t.Logf("Alice recovery = 1 tx, %d bytes, own stake %d atoms, CSV %d; zero winnings enforced", refund.SerializeSize(), refund.TxOut[0].Value, refund.TxIn[0].Sequence)
			// Older fully signed payout remains valid against the same deposits.
			old := s.draft
			old.Amounts = make([]int64, n)
			for i := range old.Amounts {
				old.Amounts[i] = 100000000
			}
			oldtx, err := BuildSettlement(old)
			if err != nil {
				t.Fatal(err)
			}
			s.draft = old
			if _, err = FinishSettlement(oldtx, old, s.sign(t, oldtx), chaincfg.MainNetParams()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStakeWarsPokerBondLadder(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := postTableBond(t, n) // owner is Bob; other keys are the remaining participants.
			d := accuseDraftFor(t, b, 1000000)
			d.FeeAtoms = auditBondFee
			ladder, err := BuildAccuseChain(d, AccuseDepth)
			if err != nil {
				t.Fatal(err)
			}
			// All signatures gathered during setup, before Bob disappears.
			accuse := ladder[0]
			accuse.TxIn[0].SignatureScript = mustAliveSig(t, b, accuse)
			if err = execute(t, b.script, accuse, csvFlags); err != nil {
				t.Fatal(err)
			}
			claimed, err := d.ClaimedScript()
			if err != nil {
				t.Fatal(err)
			}
			terms, err := ParseClaimedBond(claimed)
			if err != nil {
				t.Fatal(err)
			}
			take, err := BuildTake(TakeDraft{Claimed: claimed, Prevout: wire.OutPoint{Hash: accuse.TxHash()}, ValueAtoms: accuse.TxOut[0].Value, PayScripts: payTo(t, n-1), FeeAtoms: auditBondFee})
			if err != nil {
				t.Fatal(err)
			}
			sigs := make([][]byte, n-1)
			for i, key := range terms.Others {
				sigs[i], err = SignClaimedSpend(take, claimed, privFor(t, b.privs, key))
				if err != nil {
					t.Fatal(err)
				}
			}
			take.TxIn[0].SignatureScript, err = TakeSigScript(claimed, sigs)
			if err != nil {
				t.Fatal(err)
			}
			if err = execute(t, claimed, take, csvFlags); err != nil {
				t.Fatal(err)
			}
			t.Logf("%d seats: unanswered bond = 2 tx (%d + %d bytes), fees %d, CSV %d after accusation; each other seat %d atoms", n, accuse.SerializeSize(), take.SerializeSize(), 2*auditBondFee, take.TxIn[0].Sequence, take.TxOut[0].Value)
			auditExport(t, fmt.Sprintf("take-%d", n), take, claimed)
			if n == 6 {
				// A second refusing player blocks collection. All five others are required.
				sigs[0] = make([]byte, SigLen)
				take.TxIn[0].SignatureScript, err = TakeSigScript(claimed, sigs)
				if err != nil {
					t.Fatal(err)
				}
				if execute(t, claimed, take, csvFlags) == nil {
					t.Fatal("four of five collected bond")
				}
			}
			// Bob refuses the GAME result but answers the CHAIN accusation. The normal
			// builder re-bonds; change its output before Bob signs, as a hostile client can.
			answer, err := BuildAnswer(AnswerDraft{Claimed: claimed, Bond: b.script, Prevout: wire.OutPoint{Hash: accuse.TxHash()}, ValueAtoms: accuse.TxOut[0].Value, FeeAtoms: auditBondFee, Params: d.Params})
			if err != nil {
				t.Fatal(err)
			}
			expectedAnswerHash := answer.TxHash()
			ownerPay, err := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).AddData(stdaddr.Hash160(b.terms.Owner)).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
			if err != nil {
				t.Fatal(err)
			}
			answer.TxOut[0].PkScript = ownerPay
			ownerSig, err := SignClaimedSpend(answer, claimed, privFor(t, b.privs, b.terms.Owner))
			if err != nil {
				t.Fatal(err)
			}
			answer.TxIn[0].SignatureScript, err = AnswerSigScript(claimed, ownerSig)
			if err != nil {
				t.Fatal(err)
			}
			if err = execute(t, claimed, answer, csvFlags); err != nil {
				t.Fatalf("counterexample no longer holds: %v", err)
			}
			if answer.TxHash() == expectedAnswerHash || ladder[1].TxIn[0].PreviousOutPoint.Hash == answer.TxHash() {
				t.Fatal("unexpected ladder linkage")
			}
			_, bondPK, err := Address(b.script, d.Params)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(answer.TxOut[0].PkScript, bondPK) {
				t.Fatal("counterexample still pays bond")
			}
			auditExport(t, fmt.Sprintf("redirect-%d", n), answer, claimed)
			t.Logf("owner redirects answer immediately: %d bytes, %d atoms to owner, next presigned rung invalidated", answer.SerializeSize(), answer.TxOut[0].Value)
		})
	}
}

func auditExport(t *testing.T, name string, tx *wire.MsgTx, redeem []byte) {
	t.Helper()
	dir := os.Getenv("POKER_EXIT_FIXTURES")
	if dir == "" {
		return
	}
	raw, err := tx.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	_, pk, err := Address(redeem, chaincfg.MainNetParams())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name+".tx"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name+".script"), pk, 0600); err != nil {
		t.Fatal(err)
	}
}
