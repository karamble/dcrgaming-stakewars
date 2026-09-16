// Research variant: the answer requires all members, with the non-owner
// signatures durably collected BEFORE play. This is not Poker's current script.
package escrow

import (
	"bytes"
	"fmt"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

func auditGuardedClaim(t *testing.T, b *tableBond) []byte {
	t.Helper()
	sb := txscript.NewScriptBuilder().AddOp(txscript.OP_IF)
	for _, key := range b.terms.Members {
		sb.AddData(key).AddInt64(sigType).AddOp(txscript.OP_CHECKSIGALTVERIFY)
	}
	sb.AddOp(txscript.OP_ELSE).AddInt64(int64(ClaimBlocks)).AddOp(txscript.OP_CHECKSEQUENCEVERIFY).AddOp(txscript.OP_DROP)
	for _, key := range withoutKey(b.terms.Members, b.terms.Owner) {
		sb.AddData(key).AddInt64(sigType).AddOp(txscript.OP_CHECKSIGALTVERIFY)
	}
	script, err := sb.AddOp(txscript.OP_ENDIF).AddOp(txscript.OP_TRUE).Script()
	if err != nil {
		t.Fatal(err)
	}
	return script
}

func TestStakeWarsPokerPresignedAnswerRepair(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := postTableBond(t, n)
			d := accuseDraftFor(t, b, 1000000)
			d.FeeAtoms = auditBondFee
			guarded := auditGuardedClaim(t, b)
			_, guardPK, err := Address(guarded, d.Params)
			if err != nil {
				t.Fatal(err)
			}
			_, bondPK, err := Address(b.script, d.Params)
			if err != nil {
				t.Fatal(err)
			}
			var next = d.Prevout
			value := d.ValueAtoms
			// Precompute every exact transaction and collect the non-owner answer
			// signatures while all peers are still cooperating.
			accusations := make([]*wire.MsgTx, AccuseDepth)
			answers := make([]*wire.MsgTx, AccuseDepth)
			retained := make([][][]byte, AccuseDepth)
			ownerIndex := -1
			for i, key := range b.terms.Members {
				if bytes.Equal(key, b.terms.Owner) {
					ownerIndex = i
				}
			}
			if ownerIndex < 0 {
				t.Fatal("missing owner")
			}
			for round := 0; round < AccuseDepth; round++ {
				accuse := wire.NewMsgTx()
				accuse.Version = 3
				accuse.AddTxIn(wire.NewTxIn(&next, value, nil))
				accuse.AddTxOut(wire.NewTxOut(value-auditBondFee, guardPK))
				accuse.TxIn[0].SignatureScript = mustAliveSig(t, b, accuse)
				answer := wire.NewMsgTx()
				answer.Version = 3
				op := wire.OutPoint{Hash: accuse.TxHash()}
				answer.AddTxIn(wire.NewTxIn(&op, value-auditBondFee, nil))
				answer.AddTxOut(wire.NewTxOut(value-2*auditBondFee, bondPK))
				retained[round] = make([][]byte, n)
				for i, key := range b.terms.Members {
					if i != ownerIndex {
						retained[round][i] = signInput(t, privFor(t, b.privs, key), guarded, answer)
					}
				}
				accusations[round], answers[round] = accuse, answer
				next = wire.OutPoint{Hash: answer.TxHash()}
				value -= 2 * auditBondFee
			}
			// After setup, answering uses only the owner's key and the retained bytes.
			ownerKey := privFor(t, b.privs, b.terms.Owner)
			for round := range answers {
				answer := answers[round]
				sigs := retained[round]
				if err = execute(t, b.script, accusations[round], csvFlags); err != nil {
					t.Fatal(err)
				}
				sigs[ownerIndex] = signInput(t, ownerKey, guarded, answer)
				answer.TxIn[0].SignatureScript, err = branchSigScript(guarded, sigs, txscript.OP_1)
				if err != nil {
					t.Fatal(err)
				}
				if err = execute(t, guarded, answer, csvFlags); err != nil {
					t.Fatal(err)
				}
				if round+1 < len(answers) && accusations[round+1].TxIn[0].PreviousOutPoint.Hash != answer.TxHash() {
					t.Fatal("broken presigned lineage")
				}
				if round == 0 {
					auditExport(t, fmt.Sprintf("guarded-answer-%d", n), answer, guarded)
					// The alternative is still N-1, not a majority vote.
					take := wire.NewMsgTx()
					take.Version = 3
					claimOut := wire.OutPoint{Hash: accusations[round].TxHash()}
					take.AddTxIn(wire.NewTxIn(&claimOut, 990000, nil))
					take.TxIn[0].Sequence = ClaimBlocks
					others := withoutKey(b.terms.Members, b.terms.Owner)
					for _, pk := range payTo(t, n-1) {
						take.AddTxOut(wire.NewTxOut(980000/int64(n-1), pk))
					}
					takeSigs := make([][]byte, len(others))
					for i, key := range others {
						takeSigs[i] = signInput(t, privFor(t, b.privs, key), guarded, take)
					}
					take.TxIn[0].SignatureScript, err = branchSigScript(guarded, takeSigs, txscript.OP_0)
					if err != nil {
						t.Fatal(err)
					}
					if err = execute(t, guarded, take, csvFlags); err != nil {
						t.Fatal(err)
					}
					auditExport(t, fmt.Sprintf("guarded-take-%d", n), take, guarded)
					takeSigs[0] = make([]byte, SigLen)
					take.TxIn[0].SignatureScript, err = branchSigScript(guarded, takeSigs, txscript.OP_0)
					if err != nil {
						t.Fatal(err)
					}
					if execute(t, guarded, take, csvFlags) == nil {
						t.Fatal("missing taker signature accepted")
					}

					hostile := answer.Copy()
					hostile.TxOut[0].PkScript = payTo(t, 1)[0]
					changed := append([][]byte(nil), sigs...)
					changed[ownerIndex] = signInput(t, ownerKey, guarded, hostile)
					hostile.TxIn[0].SignatureScript, err = branchSigScript(guarded, changed, txscript.OP_1)
					if err != nil {
						t.Fatal(err)
					}
					if execute(t, guarded, hostile, csvFlags) == nil {
						t.Fatal("redirect accepted with others' fixed signatures")
					}
					// Losing a required saved signature cannot be repaired from the owner seed.
					changed = append([][]byte(nil), sigs...)
					changed[(ownerIndex+1)%n] = make([]byte, SigLen)
					lost := answer.Copy()
					lost.TxIn[0].SignatureScript, err = branchSigScript(guarded, changed, txscript.OP_1)
					if err != nil {
						t.Fatal(err)
					}
					if execute(t, guarded, lost, csvFlags) == nil {
						t.Fatal("missing backup did not block answer")
					}
					t.Logf("%d seats: answer %d bytes, redeem %d, sigscript %d; no new peer signatures or extra transaction", n, answer.SerializeSize(), len(guarded), len(answer.TxIn[0].SignatureScript))
				}
			}
			if value != 840000 {
				t.Fatal("unexpected fee accounting")
			}
			t.Logf("all 8 accusations answered: 16 transactions, fees160000, bond remains840000; no settlement signature obtained")
		})
	}
}
