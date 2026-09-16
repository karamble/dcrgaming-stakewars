package main

import (
	"bytes"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

func TestCheckpointPaths(tst *testing.T) {
	for cur := byte(1); cur < 5; cur++ {
		for next := cur + 1; next <= 5; next++ {
			tx, r := checkpointPrepare(cur, next)
			if e := checkpointVerify(tx, r); e != nil {
				tst.Fatalf("%d->%d: %v", cur, next, e)
			}
			if len(tx.TxIn[0].SignatureScript) > 1650 {
				tst.Fatal("nonstandard update")
			}
			expectedFee := int64(next-cur) * 20000
			if tx.TxIn[0].ValueIn-tx.TxOut[0].Value != expectedFee {
				tst.Fatal("fee")
			}
		}
	}
	for cur := byte(2); cur <= 5; cur++ {
		tx, r := checkpointPrepare(cur, 0)
		if e := checkpointVerify(tx, r); e != nil {
			tst.Fatal(e)
		}
		if tx.TxIn[0].ValueIn-tx.TxOut[0].Value != 20000 {
			tst.Fatal("exit fee")
		}
	}
}
func TestAcknowledgedCheckpointResumesWithoutLoser(tst *testing.T) {
	// Dictionary: index3 is B's turn after A1; index4 is A's turn after B1.
	claim, cr := checkpointPrepare(1, 3)
	if e := checkpointVerify(claim, cr); e != nil {
		tst.Fatal(e)
	}
	update, ur := checkpointPrepare(3, 4)
	update.TxIn[0].PreviousOutPoint.Hash = claim.TxHash()
	checkpointAuthorize(update, ur, ur, 4, secrets[0][2][:], secrets[1][2][:])
	if e := checkpointVerify(update, ur); e != nil {
		tst.Fatal(e)
	}
	latest, lr := checkpointPrepare(4, 0)
	latest.TxIn[0].PreviousOutPoint.Hash = update.TxHash()
	checkpointAuthorize(latest, lr, lr, 0, nil, nil)
	if e := checkpointVerify(latest, lr); e != nil {
		tst.Fatal(e)
	}
	final, fr := prepare(1, 1, 3, 1)
	final.TxIn[0].PreviousOutPoint.Hash = latest.TxHash()
	witness(final, fr, fr, keys[0], 1)
	if e := checkpointVerify(final, fr); e != nil {
		tst.Fatal(e)
	}
	if !bytes.Equal(latest.TxOut[0].PkScript, p2sh(fr)) || latest.TxOut[0].Value != final.TxIn[0].ValueIn {
		tst.Fatal("resumed state not funded exactly")
	}
	if !bytes.Equal(final.TxOut[0].PkScript, p2pk(keys[0])) {
		tst.Fatal("wrong winner")
	}
	// B's previously legal alternate move is not a spend of A's resumed snapshot.
	alt, ar := prepare(2, 2, 2, 2)
	alt.TxIn[0].PreviousOutPoint.Hash = latest.TxHash()
	witness(alt, ar, ar, keys[1], 2)
	vm, e := t.NewEngine(latest.TxOut[0].PkScript, alt, 0, t.ScriptVerifyCleanStack|t.ScriptVerifyCheckSequenceVerify, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	if e == nil {
		tst.Fatal("stale B2 accepted against latest snapshot")
	}
}
func TestCheckpointAttacks(tst *testing.T) {
	cases := []struct {
		name   string
		mutate func(*wire.MsgTx, []byte)
	}{
		{"amount", func(tx *wire.MsgTx, r []byte) {
			tx.TxOut[0].Value++
			checkpointAuthorize(tx, r, r, 4, secrets[0][2][:], secrets[1][2][:])
		}},
		{"redirect", func(tx *wire.MsgTx, r []byte) {
			tx.TxOut[0].PkScript = p2pk(keys[1])
			checkpointAuthorize(tx, r, r, 4, secrets[0][2][:], secrets[1][2][:])
		}},
		{"same sequence", func(tx *wire.MsgTx, r []byte) {
			tx.TxOut[0].PkScript = p2sh(checkpoint(3))
			tx.TxOut[0].Value = 1020000
			checkpointAuthorize(tx, r, r, 3, secrets[0][1][:], secrets[1][1][:])
		}},
		{"lower sequence", func(tx *wire.MsgTx, r []byte) {
			tx.TxOut[0].PkScript = p2sh(checkpoint(2))
			tx.TxOut[0].Value = 1040000
			checkpointAuthorize(tx, r, r, 2, secrets[0][0][:], secrets[1][0][:])
		}},
		{"mixed certificates", func(tx *wire.MsgTx, r []byte) { checkpointAuthorize(tx, r, r, 4, secrets[0][1][:], secrets[1][2][:]) }},
		{"missing loser certificate", func(tx *wire.MsgTx, r []byte) { checkpointAuthorize(tx, r, r, 4, secrets[0][2][:], nil) }},
		{"forged current state", func(tx *wire.MsgTx, r []byte) {
			checkpointAuthorize(tx, r, checkpoint(2), 4, secrets[0][2][:], secrets[1][2][:])
		}},
		{"forged suffix", func(tx *wire.MsgTx, r []byte) {
			fake := append([]byte{}, r...)
			fake[len(fake)-1] = t.OP_FALSE
			next := append([]byte{}, fake...)
			next[32] = 4
			tx.TxOut[0].PkScript = p2sh(next)
			checkpointAuthorize(tx, r, fake, 4, secrets[0][2][:], secrets[1][2][:])
		}},
		{"expiry", func(tx *wire.MsgTx, r []byte) {
			tx.Expiry = 1
			checkpointAuthorize(tx, r, r, 4, secrets[0][2][:], secrets[1][2][:])
		}},
		{"forged destination dictionary", func(tx *wire.MsgTx, r []byte) {
			bad := publicDictionary()
			bad[len(bad)-1] ^= 1
			checkpointAuthorize(tx, r, r, 4, secrets[0][2][:], secrets[1][2][:])
			var e error
			tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(bad).AddData(secrets[0][2][:]).AddData(secrets[1][2][:]).AddInt64(4).AddData(prefix(tx)).AddData(r).AddData(r).Script()
			must(e)
		}},
		{"wrong resumed state", func(tx *wire.MsgTx, r []byte) {
			tx.TxOut[0].PkScript = p2sh(redeem(1, 1, 3))
			tx.TxOut[0].Value = 1000000
			tx.TxIn[0].Sequence = 2
			checkpointAuthorize(tx, r, r, 0, nil, nil)
		}},
	}
	for _, tt := range cases {
		tst.Run(tt.name, func(tst *testing.T) {
			tx, r := checkpointPrepare(3, 4)
			tt.mutate(tx, r)
			if e := checkpointVerify(tx, r); e == nil {
				tst.Fatal("attack accepted")
			}
		})
	}
	tx, r := checkpointPrepare(1, 2)
	tx.TxIn[0].Sequence = 2
	tx.TxOut[0].PkScript = p2sh(redeem(3, 1, 1))
	tx.TxOut[0].Value = 1040000
	checkpointAuthorize(tx, r, r, 0, nil, nil)
	if e := checkpointVerify(tx, r); e == nil {
		tst.Fatal("funding bypasses fresh claim")
	}
	tx, r = checkpointPrepare(4, 0)
	tx.TxIn[0].Sequence = 1
	checkpointAuthorize(tx, r, r, 0, nil, nil)
	if e := checkpointVerify(tx, r); e == nil {
		tst.Fatal("early exit")
	}
	tx, r = checkpointPrepare(4, 0)
	tx.TxIn[0].Sequence = 0x80000002
	checkpointAuthorize(tx, r, r, 0, nil, nil)
	if e := checkpointVerify(tx, r); e == nil {
		tst.Fatal("disabled CSV exit")
	}
	tx, r = checkpointPrepare(4, 0)
	var e error
	tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(publicDictionary()).AddData(nil).AddData(nil).AddInt64(-1).AddData(prefix(tx)).AddData(r).AddData(r).Script()
	must(e)
	if e = checkpointVerify(tx, r); e == nil {
		tst.Fatal("negative command accepted")
	}
}
