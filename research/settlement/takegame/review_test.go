package main

import (
	"bytes"
	"github.com/decred/dcrd/dcrec"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/sign"
	"github.com/decred/dcrd/wire"
	"testing"
)

func evaluate(tx *wire.MsgTx, r []byte) error {
	flags := t.ScriptVerifyCleanStack | t.ScriptVerifySigPushOnly | t.ScriptVerifyCheckSequenceVerify | t.ScriptVerifyCheckLockTimeVerify | t.ScriptVerifySHA256 | t.ScriptVerifyTreasury | t.ScriptDiscourageUpgradableNops
	vm, e := t.NewEngine(p2sh(r), tx, 0, flags, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	return e
}

func TestIndependentChoiceMutations(tst *testing.T) {
	cases := []struct {
		name   string
		mutate func(*wire.MsgTx, []byte)
		action int64
		key    int
	}{
		{"early_draw", func(tx *wire.MsgTx, r []byte) {
			v := tx.TxOut[0].Value
			tx.TxOut = nil
			tx.AddTxOut(wire.NewTxOut(v/2, p2pk(keys[0])))
			tx.AddTxOut(wire.NewTxOut(v/2, p2pk(keys[1])))
		}, 1, 0},
		{"skip_changes_pile", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(2, 2, 2)); tx.TxIn[0].Sequence = 2 }, 0, 1},
		{"skip_same_seat", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(3, 1, 2)); tx.TxIn[0].Sequence = 2 }, 0, 1},
		{"skip_same_turn", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(3, 2, 1)); tx.TxIn[0].Sequence = 2 }, 0, 1},
		{"skip_jump_to_final_turn", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(3, 2, 8)); tx.TxIn[0].Sequence = 2 }, 0, 1},
		{"action_changes_turn", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(2, 2, 3)) }, 1, 0},
		{"action_changes_seat", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(redeem(2, 1, 2)) }, 1, 0},
		{"action_two_with_one_output", func(tx *wire.MsgTx, r []byte) {}, 2, 0},
		{"expiry_high_byte", func(tx *wire.MsgTx, r []byte) { tx.Expiry = 1 << 24 }, 1, 0},
		{"locktime_high_byte", func(tx *wire.MsgTx, r []byte) {}, 1, 0},
		{"wrong_output_version", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].Version = 1 }, 1, 0},
		{"extra_output", func(tx *wire.MsgTx, r []byte) { tx.AddTxOut(wire.NewTxOut(1000, p2pk(keys[1]))) }, 1, 0},
	}
	for _, c := range cases {
		tst.Run(c.name, func(tst *testing.T) {
			tx, r := prepare(3, 1, 1, 1)
			c.mutate(tx, r)
			witness(tx, r, r, keys[c.key], c.action)
			if c.name == "locktime_high_byte" {
				tx.LockTime = 1 << 24
				h, _ := sign.RawTxInSignature(tx, 0, r, t.SigHashAll, keys[0].Serialize(), dcrec.STEcdsaSecp256k1)
				tx.TxIn[0].SignatureScript, _ = t.NewScriptBuilder().AddData(h).AddInt64(1).AddData(prefix(tx)).AddData(r).AddData(r).Script()
			}
			if e := evaluate(tx, r); e == nil {
				tst.Fatal("mutation accepted")
			}
		})
	}
}

func TestIndependentSignerAndHashType(tst *testing.T) {
	for _, ht := range []t.SigHashType{t.SigHashNone, t.SigHashSingle, t.SigHashAll | t.SigHashAnyOneCanPay} {
		tx, r := prepare(3, 1, 1, 1)
		s, e := sign.RawTxInSignature(tx, 0, r, ht, keys[0].Serialize(), dcrec.STEcdsaSecp256k1)
		if e != nil {
			tst.Fatal(e)
		}
		tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(s).AddInt64(1).AddData(prefix(tx)).AddData(r).AddData(r).Script()
		if e != nil {
			tst.Fatal(e)
		}
		if e = evaluate(tx, r); e == nil {
			tst.Fatalf("hash type %x accepted", ht)
		}
	}
	tx, r := prepare(3, 1, 1, 1)
	// The known synthetic private scalar is n-1. It must not authorize play.
	var nMinus1 secp256k1.ModNScalar
	nMinus1.SetInt(1)
	nMinus1.Negate()
	k := secp256k1.NewPrivateKey(&nMinus1)
	witness(tx, r, r, k, 1)
	if e := evaluate(tx, r); e == nil {
		tst.Fatal("synthetic private scalar authorized action")
	}
}

func TestNoncanonicalActionEncoding(tst *testing.T) {
	for _, encoded := range [][]byte{{1, 0}, {0x80}, {0, 0}, {0xff, 0xff, 0xff, 0x7f}} {
		tx, r := prepare(3, 1, 1, 1)
		sig, e := sign.RawTxInSignature(tx, 0, r, t.SigHashAll, keys[0].Serialize(), dcrec.STEcdsaSecp256k1)
		if e != nil {
			tst.Fatal(e)
		}
		tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(sig).AddData(encoded).AddData(prefix(tx)).AddData(r).AddData(r).Script()
		if e != nil {
			tst.Fatal(e)
		}
		if e = evaluate(tx, r); e == nil {
			tst.Fatalf("accepted noncanonical/out-of-range action %x", encoded)
		}
	}
}

func TestIndependentTerminalRules(tst *testing.T) {
	for _, seat := range []byte{1, 2} {
		tx, r := prepare(1, seat, 8, 1)
		if e := evaluate(tx, r); e != nil {
			tst.Fatal(e)
		}
		if len(tx.TxOut) != 1 || !bytes.Equal(tx.TxOut[0].PkScript, p2pk(keys[seat-1])) {
			tst.Fatal("last allowed move taking final object must win before draw")
		}
		tx.TxOut[0].PkScript = p2pk(keys[2-seat])
		witness(tx, r, r, keys[seat-1], 1)
		if e := evaluate(tx, r); e == nil {
			tst.Fatal("wrong terminal winner accepted")
		}
	}
	tx, r := prepare(3, 1, 8, 0)
	// No signature from either actor is required to finish a mature timeout draw.
	tx.TxIn[0].SignatureScript, _ = t.NewScriptBuilder().AddData(nil).AddInt64(0).AddData(prefix(tx)).AddData(r).AddData(r).Script()
	if e := evaluate(tx, r); e != nil {
		tst.Fatal(e)
	}
}

func TestIndependentLinkedPath(tst *testing.T) {
	// A takes 1, absent B is skipped, A takes the remaining 2 and wins.
	p, s, u := byte(3), byte(1), byte(1)
	var prior *wire.MsgTx
	for _, a := range []int64{1, 0, 2} {
		tx, r := prepare(p, s, u, a)
		if prior != nil {
			if !bytes.Equal(prior.TxOut[0].PkScript, p2sh(r)) {
				tst.Fatal("previous successor script does not match current script")
			}
			if prior.TxOut[0].Value != tx.TxIn[0].ValueIn {
				tst.Fatal("successor amount mismatch")
			}
			tx.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: prior.TxHash(), Index: 0, Tree: wire.TxTreeRegular}
			witness(tx, r, r, keys[s-1], a)
		}
		if e := evaluate(tx, r); e != nil {
			tst.Fatal(e)
		}
		prior = tx
		p -= byte(a)
		s = 3 - s
		u++
	}
	if len(prior.TxOut) != 1 || !bytes.Equal(prior.TxOut[0].PkScript, p2pk(keys[0])) {
		tst.Fatal("linked path did not pay A")
	}
	tst.Log("actual linked outpoints/scripts/value preservation verified by script engine; timeout input age requires contextual chain test")
}

// This passes when it reproduces the off-chain finality gap. Neither competing
// branch violates the toy game: absent on-chain confirmation or a channel
// dispute layer, the chain can select a branch that differs from the BR replay.
func TestOffchainRollbackCounterexample(tst *testing.T) {
	first, r := prepare(3, 1, 1, 1)
	if e := evaluate(first, r); e != nil {
		tst.Fatal(e)
	}
	branch := func(action int64) *wire.MsgTx {
		tx, r := prepare(2, 2, 2, action)
		tx.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: first.TxHash(), Index: 0, Tree: wire.TxTreeRegular}
		witness(tx, r, r, keys[1], action)
		if e := evaluate(tx, r); e != nil {
			tst.Fatal(e)
		}
		return tx
	}
	bOne := branch(1)
	aWin, r := prepare(1, 1, 3, 1)
	aWin.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: bOne.TxHash(), Index: 0, Tree: wire.TxTreeRegular}
	witness(aWin, r, r, keys[0], 1)
	if e := evaluate(aWin, r); e != nil {
		tst.Fatal(e)
	}
	if !bytes.Equal(aWin.TxOut[0].PkScript, p2pk(keys[0])) {
		tst.Fatal("first path must pay A")
	}
	// B can generate this after observing A's winning continuation. Nothing in
	// the script knows that B previously authorized a different unconfirmed move.
	bTwo := branch(2)
	if bOne.TxIn[0].PreviousOutPoint != bTwo.TxIn[0].PreviousOutPoint {
		tst.Fatal("must spend same B-turn state")
	}
	if !bytes.Equal(bTwo.TxOut[0].PkScript, p2pk(keys[1])) {
		tst.Fatal("alternative must pay B")
	}
	tst.Log("Both A1->B1->A1 (A wins) and A1->B2 (B wins) satisfy scripts; first-confirmed B spend chooses the authoritative branch, not the previously replayed BR transcript")
}
