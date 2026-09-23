// This package is an isolated research prototype, not a production contract.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/decred/dcrd/crypto/blake256"
	"github.com/decred/dcrd/crypto/ripemd160"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	wots "github.com/karamble/dcrgaming-stakewars/research/settlement/wots"
	"os"
	"path/filepath"
)

var gx, _ = hex.DecodeString("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")

func must(e error) {
	if e != nil {
		panic(e)
	}
}

type compiler struct {
	b  *t.ScriptBuilder
	st []string
}

func (c *compiler) data(n string, v []byte) { c.b.AddData(v); c.st = append(c.st, n) }
func (c *compiler) num(n string, v int64)   { c.b.AddInt64(v); c.st = append(c.st, n) }
func (c *compiler) op(n string, k int, ops ...byte) {
	for _, o := range ops {
		c.b.AddOp(o)
	}
	c.st = c.st[:len(c.st)-k]
	if n != "" {
		c.st = append(c.st, n)
	}
}
func (c *compiler) dup(n string) {
	for i := len(c.st) - 1; i >= 0; i-- {
		if c.st[i] == n {
			d := len(c.st) - 1 - i
			if d == 0 {
				c.b.AddOp(t.OP_DUP)
			} else if d == 1 {
				c.b.AddOp(t.OP_OVER)
			} else {
				c.b.AddInt64(int64(d)).AddOp(t.OP_PICK)
			}
			c.st = append(c.st, n)
			return
		}
	}
	panic("missing:" + n)
}
func (c *compiler) cat() { c.op("x", 2, t.OP_CAT) }
func p2sh(v []byte) []byte {
	h := blake256.Sum256(v)
	r := ripemd160.New()
	r.Write(h[:])
	s, e := t.NewScriptBuilder().AddOp(t.OP_HASH160).AddData(r.Sum(nil)).AddOp(t.OP_EQUAL).Script()
	must(e)
	return s
}

// emit serialize output from named pk and amount; fixed known pk size.
func (c *compiler) out(pk, val string, l byte) {
	c.dup(val)
	c.data("pad", make([]byte, 5))
	c.cat()
	c.data("version+len", []byte{0, 0, l})
	c.cat()
	c.dup(pk)
	c.cat()
}

func prefix(tx *wire.MsgTx) []byte {
	var p bytes.Buffer
	binary.Write(&p, binary.LittleEndian, uint32(tx.Version)|1<<16)
	p.WriteByte(1)
	p.Write(tx.TxIn[0].PreviousOutPoint.Hash[:])
	binary.Write(&p, binary.LittleEndian, tx.TxIn[0].PreviousOutPoint.Index)
	p.WriteByte(byte(tx.TxIn[0].PreviousOutPoint.Tree))
	binary.Write(&p, binary.LittleEndian, tx.TxIn[0].Sequence)
	p.WriteByte(byte(len(tx.TxOut)))
	for _, o := range tx.TxOut {
		binary.Write(&p, binary.LittleEndian, o.Value)
		binary.Write(&p, binary.LittleEndian, o.Version)
		wire.WriteVarInt(&p, 0, uint64(len(o.PkScript)))
		p.Write(o.PkScript)
	}
	binary.Write(&p, binary.LittleEndian, tx.LockTime)
	binary.Write(&p, binary.LittleEndian, tx.Expiry)
	return p.Bytes()
}

func wotsScript(state []byte, n int) []byte {
	c := &compiler{t.NewScriptBuilder(), []string{"prefix", "code", "opening"}}
	h := blake256.Sum256(state)
	c.dup("opening")
	c.op("h", 1, t.OP_BLAKE256)
	c.data("expected", h[:])
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("opening")
	c.op("length", 1, t.OP_SIZE, t.OP_NIP)
	c.num("len", 131)
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("opening")
	c.num("end", 91)
	c.num("start", 88)
	c.op("highstep", 3, t.OP_SUBSTR)
	c.data("zeroHighStep", []byte{0, 0, 0})
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("opening")
	c.num("end", 92)
	c.num("start", 91)
	c.op("stepbyte", 3, t.OP_SUBSTR)
	c.data("guard", []byte{1})
	c.cat()
	c.num("guardint", 256)
	c.op("step", 2, t.OP_SUB)
	c.dup("step")
	c.num("zero", 0)
	c.num("end", 15)
	c.op("", 3, t.OP_WITHIN, t.OP_VERIFY)
	c.dup("opening")
	c.num("end", 128)
	c.num("start", 96)
	c.op("X", 3, t.OP_SUBSTR)
	c.dup("opening")
	c.num("end", 64)
	c.num("start", 32)
	c.op("seed", 3, t.OP_SUBSTR)
	c.dup("opening")
	c.num("end", 96)
	c.num("start", 64)
	c.op("addr", 3, t.OP_SUBSTR)
	wots.EmitDynamicStep(c.b)
	c.st = c.st[:len(c.st)-3]
	c.st = append(c.st, "Y")
	c.dup("opening")
	c.num("start", 128)
	c.op("oldamount", 2, t.OP_RIGHT)
	c.num("fee", 20000)
	c.op("amount", 2, t.OP_SUB)
	c.dup("opening")
	c.num("end", 91)
	c.op("newstateprefix", 2, t.OP_LEFT)
	c.dup("step")
	c.op("nextstep", 1, t.OP_1ADD)
	c.cat()
	c.dup("opening")
	c.num("end", 96)
	c.num("start", 92)
	c.op("km", 3, t.OP_SUBSTR)
	c.cat()
	c.dup("Y")
	c.cat()
	c.dup("amount")
	c.cat()
	c.op("nextroot", 1, t.OP_BLAKE256)
	// Root hash occupies a fixed33-byte push after initial OP_DUP OP_BLAKE256.
	// Replace only those bytes; preserve the full remaining executable template.
	c.data("rootpush", []byte{32})
	c.b.AddOp(t.OP_SWAP)
	c.cat()
	c.dup("code")
	c.num("start", 35)
	c.op("suffix", 2, t.OP_RIGHT)
	c.cat()
	c.data("prefixops", []byte{t.OP_DUP, t.OP_BLAKE256})
	c.b.AddOp(t.OP_SWAP)
	c.cat()
	c.op("nexthash", 1, t.OP_HASH160)
	c.data("p2sh", []byte{t.OP_HASH160, 20})
	c.b.AddOp(t.OP_SWAP)
	c.cat()
	c.data("eq", []byte{t.OP_EQUAL})
	c.cat()
	c.st[len(c.st)-1] = "dest"
	c.data("count", []byte{1})
	c.out("dest", "amount", 23)
	c.cat()
	c.st[len(c.st)-1] = "outputs"
	c.dup("prefix")
	c.num("len", 5)
	c.op("header", 2, t.OP_LEFT)
	c.data("header", []byte{2, 0, 1, 0, 1})
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("prefix")
	c.dup("prefix")
	c.op("size", 1, t.OP_SIZE, t.OP_NIP)
	c.num("tail", 7)
	c.op("idx", 2, t.OP_SUB)
	c.op("tail", 2, t.OP_RIGHT)
	c.data("zeros", make([]byte, 7))
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("prefix")
	c.dup("prefix")
	c.op("size", 1, t.OP_SIZE, t.OP_NIP)
	c.num("tail", 8)
	c.op("end", 2, t.OP_SUB)
	c.num("start", 46)
	c.op("actualoutputs", 3, t.OP_SUBSTR)
	c.dup("outputs")
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("prefix")
	c.op("ph", 1, t.OP_BLAKE256)
	var wh bytes.Buffer
	binary.Write(&wh, binary.LittleEndian, uint32(2|3<<16))
	wh.WriteByte(1)
	wire.WriteVarInt(&wh, 0, uint64(n))
	c.data("whheader", wh.Bytes())
	c.dup("code")
	c.cat()
	c.op("wh", 1, t.OP_BLAKE256)
	c.data("hashtype", []byte{1, 0, 0, 0})
	c.dup("ph")
	c.cat()
	c.dup("wh")
	c.cat()
	c.op("message", 1, t.OP_BLAKE256)
	c.b.AddData(gx).AddOp(t.OP_DUP).AddOp(t.OP_TOALTSTACK).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256).AddOp(t.OP_DUP).AddInt64(31).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddInt64(31).AddOp(t.OP_RIGHT).AddOp(t.OP_DUP).AddInt64(1).AddInt64(127).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY).AddOp(t.OP_1ADD).AddOp(t.OP_CAT).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_DUP).AddOp(t.OP_TOALTSTACK).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{1}).AddOp(t.OP_CAT).AddOp(t.OP_FROMALTSTACK).AddData([]byte{3}).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddInt64(2).AddOp(t.OP_CHECKSIGALTVERIFY)
	c.st = c.st[:len(c.st)-1]
	for len(c.st) > 0 {
		if len(c.st) > 1 {
			c.op("", 2, t.OP_2DROP)
		} else {
			c.op("", 1, t.OP_DROP)
		}
	}
	c.b.AddOp(t.OP_TRUE)
	v, e := c.b.Script()
	must(e)
	return v
}
func wotsRedeem(st []byte) []byte {
	v := wotsScript(st, 0)
	for i := 0; i < 3; i++ {
		v = wotsScript(st, len(v))
	}
	return v
}
func wotsWitness(tx *wire.MsgTx, r, opening []byte) {
	tx.LockTime = 0
	for {
		tx.LockTime++
		h, e := t.CalcSignatureHash(r, t.SigHashAll, tx, 0, nil)
		must(e)
		x := blake256.Sum256(append(append([]byte{}, gx...), h...))
		if x[31] >= 1 && x[31] < 127 {
			break
		}
	}
	var e error
	tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(prefix(tx)).AddData(r).AddData(opening).AddData(r).Script()
	must(e)
}

func initialOpening() []byte {
	st := make([]byte, 131)
	msg := blake256.Sum256([]byte("arbitrary256bitcheckpoint"))
	copy(st, msg[:])
	seed := blake256.Sum256([]byte("publicseed"))
	copy(st[32:], seed[:])
	st[83] = 2
	st[87] = 7
	X := blake256.Sum256([]byte("chainstart"))
	copy(st[96:], X[:])
	setAmount(st, 3000000)
	return st
}
func amount(st []byte) int64 { return int64(st[128]) | int64(st[129])<<8 | int64(st[130])<<16 }
func setAmount(st []byte, v int64) {
	st[128] = byte(v)
	st[129] = byte(v >> 8)
	st[130] = byte(v >> 16)
}
func fixture(st []byte) (*wire.MsgTx, []byte, []byte) {
	r := wotsRedeem(st)
	next := append([]byte{}, st...)
	var X, seed, address [32]byte
	copy(X[:], st[96:128])
	copy(seed[:], st[32:64])
	copy(address[:], st[64:96])
	Y := wots.ReferenceStep(X, seed, address)
	copy(next[96:], Y[:])
	next[91]++
	setAmount(next, amount(st)-20000)
	tx := wire.NewMsgTx()
	tx.Version = 2
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, amount(st), nil))
	tx.TxIn[0].Sequence = 0xfffffffe
	tx.AddTxOut(wire.NewTxOut(amount(next), p2sh(wotsRedeem(next))))
	wotsWitness(tx, r, st)
	return tx, r, next
}
func check(tx *wire.MsgTx, r []byte) error {
	vm, e := t.NewEngine(p2sh(r), tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySHA256|t.ScriptVerifySigPushOnly, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	return e
}
func countOps(r []byte) int {
	n := 0
	tok := t.MakeScriptTokenizer(0, r)
	for tok.Next() {
		if tok.Opcode() > t.OP_16 {
			n++
		}
	}
	must(tok.Err())
	return n
}
func export(dir, name string, tx *wire.MsgTx, r []byte) {
	must(os.MkdirAll(dir, 0700))
	b, e := tx.Bytes()
	must(e)
	must(os.WriteFile(filepath.Join(dir, name+".tx"), b, 0600))
	must(os.WriteFile(filepath.Join(dir, name+"-prevout.script"), p2sh(r), 0600))
}
func main() {
	dir := flag.String("export", "", "optional fixture directory")
	flag.Parse()
	st := initialOpening()
	tx, r, next := fixture(st)
	must(check(tx, r))
	fmt.Printf("pot-bound RFC8391 step: redeem%d ops%d sigscript%d tx%d fee20000\n", len(r), countOps(r), len(tx.TxIn[0].SignatureScript), tx.SerializeSize())
	child, cr, _ := fixture(next)
	child.TxIn[0].PreviousOutPoint.Hash = tx.TxHash()
	wotsWitness(child, cr, next)
	must(check(child, cr))
	if *dir != "" {
		export(*dir, "valid", tx, r)
		export(*dir, "linked", child, cr)
	}
}
