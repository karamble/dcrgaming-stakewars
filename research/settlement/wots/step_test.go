package wotsprobe

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

func TestRFC8391Step(tst *testing.T) {
	for i := 0; i < 100; i++ {
		x := sha256.Sum256([]byte(fmt.Sprintf("input%d", i)))
		seed := sha256.Sum256([]byte("public-seed"))
		var address [32]byte
		binary.BigEndian.PutUint32(address[20:24], 7)
		binary.BigEndian.PutUint32(address[24:28], uint32(i%15))
		b := t.NewScriptBuilder()
		EmitStep(b, seed, address)
		expected := ReferenceStep(x, seed, address)
		b.AddData(expected[:]).AddOp(t.OP_EQUAL)
		script, e := b.Script()
		if e != nil {
			tst.Fatal(e)
		}
		if i == 0 {
			tst.Logf("bytes=%d ops=%d", len(script), countOps(script))
		}
		if e = execute(script, x); e != nil {
			tst.Fatal(i, e)
		}
	}
}

func TestDynamicRFC8391Step(tst *testing.T) {
	for i := 0; i < 100; i++ {
		x := sha256.Sum256([]byte(fmt.Sprintf("input%d", i)))
		seed := sha256.Sum256([]byte("public-seed"))
		var address [32]byte
		binary.BigEndian.PutUint32(address[20:24], uint32(i%67))
		binary.BigEndian.PutUint32(address[24:28], uint32(i%15))
		b := t.NewScriptBuilder()
		EmitDynamicStep(b)
		expected := ReferenceStep(x, seed, address)
		b.AddData(expected[:]).AddOp(t.OP_EQUAL)
		script, e := b.Script()
		if e != nil {
			tst.Fatal(e)
		}
		if i == 0 {
			tst.Logf("dynamic+comparison bytes=%d ops=%d", len(script), countOps(script))
		}
		sig, e := t.NewScriptBuilder().AddData(x[:]).AddData(seed[:]).AddData(address[:]).Script()
		if e != nil {
			tst.Fatal(e)
		}
		tx := wire.NewMsgTx()
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, 100000, sig))
		tx.AddTxOut(wire.NewTxOut(90000, []byte{t.OP_TRUE}))
		vm, e := t.NewEngine(script, tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySHA256, 0, nil)
		if e == nil {
			e = vm.Execute()
		}
		if e != nil {
			tst.Fatal(i, e)
		}
	}
}

func TestZeroMaskResultEncoding(tst *testing.T) {
	seed := sha256.Sum256([]byte("public-seed"))
	var address [32]byte
	var input [96]byte
	input[31] = 3
	copy(input[32:64], seed[:])
	input[95] = 1
	// X equal to the mask makes every XOR chunk zero. ScriptNum encodes zero
	// as an empty vector; the emitter must restore all32 zero bytes for F.
	x := sha256.Sum256(input[:])
	expected := ReferenceStep(x, seed, address)
	b := t.NewScriptBuilder()
	EmitStep(b, seed, address)
	b.AddData(expected[:]).AddOp(t.OP_EQUAL)
	script, err := b.Script()
	if err != nil {
		tst.Fatal(err)
	}
	if err = execute(script, x); err != nil {
		tst.Fatal(err)
	}
}

func countOps(script []byte) int {
	n := 0
	tok := t.MakeScriptTokenizer(0, script)
	for tok.Next() {
		if tok.Opcode() > t.OP_16 {
			n++
		}
	}
	return n
}
func execute(script []byte, x [32]byte) error {
	sig, e := t.NewScriptBuilder().AddData(x[:]).Script()
	if e != nil {
		return e
	}
	tx := wire.NewMsgTx()
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, 100000, sig))
	tx.AddTxOut(wire.NewTxOut(90000, []byte{t.OP_TRUE}))
	vm, e := t.NewEngine(script, tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySHA256, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	return e
}
