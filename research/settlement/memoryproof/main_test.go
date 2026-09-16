package main

import (
	"bytes"
	"encoding/binary"
	"github.com/decred/dcrd/crypto/blake256"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

func arguments(tx *wire.MsgTx) [][]byte {
	var v [][]byte
	tok := t.MakeScriptTokenizer(0, tx.TxIn[0].SignatureScript)
	for tok.Next() {
		v = append(v, append([]byte(nil), tok.Data()...))
	}
	if tok.Err() != nil {
		panic(tok.Err())
	}
	return v
}
func setArguments(tx *wire.MsgTx, v [][]byte) {
	b := t.NewScriptBuilder()
	for _, a := range v {
		b.AddData(a)
	}
	s, e := b.Script()
	must(e)
	tx.TxIn[0].SignatureScript = s
}

// Recreate a supplied prefix and witness hash for the actual altered transaction.
// This ensures rejection is not merely because the old witness became stale.
func refresh(tx *wire.MsgTx, redeem []byte) {
	for {
		tx.LockTime++
		prefix, e := tx.BytesPrefix()
		must(e)
		var w bytes.Buffer
		binary.Write(&w, binary.LittleEndian, uint32(tx.Version)|3<<16)
		wire.WriteVarInt(&w, 0, uint64(len(tx.TxIn)))
		for i := range tx.TxIn {
			if i == 0 {
				wire.WriteVarInt(&w, 0, uint64(len(redeem)))
				w.Write(redeem)
			} else {
				w.WriteByte(0)
			}
		}
		wh := blake256.Sum256(w.Bytes())
		h, e := t.CalcSignatureHash(redeem, t.SigHashAll, tx, 0, nil)
		must(e)
		gx := []byte{0x79, 0xbe, 0x66, 0x7e, 0xf9, 0xdc, 0xbb, 0xac, 0x55, 0xa0, 0x62, 0x95, 0xce, 0x87, 0x0b, 0x07, 0x02, 0x9b, 0xfc, 0xdb, 0x2d, 0xce, 0x28, 0xd9, 0x59, 0xf2, 0x81, 0x5b, 0x16, 0xf8, 0x17, 0x98}
		challenge := blake256.Sum256(append(gx, h...))
		if challenge[31] >= 1 && challenge[31] < 127 {
			args := arguments(tx)
			args[0], args[1] = wh[:], prefix
			setArguments(tx, args)
			return
		}
	}
}
func TestFixedOutputCovenant(tst *testing.T) {
	tx, pk, _ := fixture(108, -1)
	if e := check(tx, pk); e != nil {
		tst.Fatal(e)
	}
	if n := len(tx.TxIn[0].SignatureScript); n > 1650 {
		tst.Fatalf("signature script size %d", n)
	}
	if n := t.GetPreciseSigOpCount(tx.TxIn[0].SignatureScript, pk, true); n != 1 {
		tst.Fatalf("sigops %d", n)
	}
}
func TestCovenantRejectsAlteredTransactions(tst *testing.T) {
	tests := []struct {
		name   string
		change func(*wire.MsgTx)
	}{
		{"amount", func(tx *wire.MsgTx) { tx.TxOut[0].Value++ }},
		{"destination", func(tx *wire.MsgTx) { tx.TxOut[0].PkScript[3] ^= 1 }},
		{"additional output", func(tx *wire.MsgTx) { tx.AddTxOut(wire.NewTxOut(1000, []byte{t.OP_TRUE})) }},
		{"additional input", func(tx *wire.MsgTx) { tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: 1}, 1000, nil)) }},
		{"script version", func(tx *wire.MsgTx) { tx.TxOut[0].Version = 1 }},
		{"transaction expiry", func(tx *wire.MsgTx) { tx.Expiry = 1000000 }},
		{"nonfinal sequence", func(tx *wire.MsgTx) { tx.TxIn[0].Sequence = 1 }},
	}
	for _, tt := range tests {
		tst.Run(tt.name, func(tst *testing.T) {
			tx, pk, r := fixture(108, -1)
			tt.change(tx)
			refresh(tx, r)
			if e := check(tx, pk); e == nil {
				tst.Fatal("invalid transaction accepted")
			}
		})
	}
}
func TestCovenantRejectsForgedArguments(tst *testing.T) {
	tests := []struct {
		name   string
		change func([][]byte) [][]byte
	}{
		{"forged prefix", func(v [][]byte) [][]byte { v[1][5] ^= 1; return v }},
		{"forged witness hash", func(v [][]byte) [][]byte { v[0][0] ^= 1; return v }},
		{"short witness hash", func(v [][]byte) [][]byte { v[0] = v[0][:31]; return v }},
		{"long prefix", func(v [][]byte) [][]byte { v[1] = append(v[1], 0); return v }},
		{"noncanonical input count", func(v [][]byte) [][]byte {
			v[1] = append(append(append([]byte{}, v[1][:4]...), 0xfd, 1, 0), v[1][5:]...)
			return v
		}},
		{"extra signature argument", func(v [][]byte) [][]byte { return append([][]byte{bytes.Repeat([]byte{1}, 65)}, v...) }},
	}
	for _, tt := range tests {
		tst.Run(tt.name, func(tst *testing.T) {
			tx, pk, _ := fixture(108, -1)
			setArguments(tx, tt.change(arguments(tx)))
			if e := check(tx, pk); e == nil {
				tst.Fatal("forged witness accepted")
			}
		})
	}
}

func TestMemoryArithmeticFraud(tst *testing.T) {
	tx, pk, _ := fixture(int64(address%100+1+7), -1)
	if err := check(tx, pk); err == nil {
		tst.Fatal("honest arithmetic claim can be challenged")
	}
	for level := 0; level < depth; level++ {
		tx, pk, _ := fixture(108, level)
		if err := check(tx, pk); err == nil {
			tst.Fatalf("corrupt sibling %d accepted", level)
		}
	}
	tx, pk, _ = fixture(108, -1)
	args := arguments(tx)
	args[len(args)-2] = []byte{19}
	setArguments(tx, args)
	if err := check(tx, pk); err == nil {
		tst.Fatal("wrong memory cell accepted")
	}
}

func TestMalformedMemoryPath(tst *testing.T) {
	for _, n := range []int{0, 31, 33, 64} {
		tx, pk, _ := fixture(108, -1)
		args := arguments(tx)
		args[2] = make([]byte, n)
		setArguments(tx, args)
		if err := check(tx, pk); err == nil {
			tst.Fatalf("%d-byte sibling accepted", n)
		}
	}
}

func TestOperationBudget(tst *testing.T) {
	_, _, redeem := fixture(108, -1)
	tok := t.MakeScriptTokenizer(0, redeem)
	ops := 0
	for tok.Next() {
		if tok.Opcode() > t.OP_16 {
			ops++
		}
	}
	if tok.Err() != nil {
		tst.Fatal(tok.Err())
	}
	if ops > 255 {
		tst.Fatalf("%d operations exceed limit", ops)
	}
	tst.Logf("memory fraud branch: %d operations", ops)
}
