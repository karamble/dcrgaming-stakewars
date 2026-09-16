package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/decred/dcrd/crypto/blake256"
	"github.com/decred/dcrd/crypto/ripemd160"
	"github.com/decred/dcrd/dcrec"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/sign"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
)

var gx, _ = hex.DecodeString("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
var keys = []*secp256k1.PrivateKey{secp256k1.PrivKeyFromBytes([]byte{42}), secp256k1.PrivKeyFromBytes([]byte{44})}

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
func (c *compiler) choose(n string, yes, no func()) {
	c.op("", 1, t.OP_IF)
	base := append([]string{}, c.st...)
	normalize := func() {
		c.op("", 1, t.OP_TOALTSTACK)
		for len(c.st) > len(base) {
			if len(c.st)-len(base) >= 2 {
				c.op("", 2, t.OP_2DROP)
			} else {
				c.op("", 1, t.OP_DROP)
			}
		}
		c.b.AddOp(t.OP_FROMALTSTACK)
		c.st = append(c.st, n)
	}
	yes()
	normalize()
	c.b.AddOp(t.OP_ELSE)
	c.st = append([]string{}, base...)
	no()
	normalize()
	c.b.AddOp(t.OP_ENDIF)
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
func p2pk(k *secp256k1.PrivateKey) []byte {
	s, e := t.NewScriptBuilder().AddData(k.PubKey().SerializeCompressed()).AddOp(t.OP_CHECKSIG).Script()
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
func script(pile, seat, turn byte, n int) []byte {
	c := &compiler{t.NewScriptBuilder(), []string{"sig", "action", "prefix", "code"}}
	state := make([]byte, 32)
	state[29] = pile
	state[30] = seat
	state[31] = turn
	c.data("state", state)
	c.dup("state")
	c.num("end", 30)
	c.num("start", 29)
	c.op("pile", 3, t.OP_SUBSTR)
	c.dup("state")
	c.num("end", 31)
	c.num("start", 30)
	c.op("seat", 3, t.OP_SUBSTR)
	c.dup("state")
	c.num("start", 31)
	c.op("turn", 2, t.OP_RIGHT)
	c.dup("action")
	c.num("min", 0)
	c.num("end", 3)
	c.op("", 3, t.OP_WITHIN, t.OP_VERIFY)
	c.dup("action")
	c.dup("pile")
	c.op("", 2, t.OP_LESSTHANOREQUAL, t.OP_VERIFY)
	c.dup("seat")
	c.num("one", 1)
	c.op("cond", 2, t.OP_EQUAL)
	c.choose("actor", func() { c.data("actor", keys[0].PubKey().SerializeCompressed()) }, func() { c.data("actor", keys[1].PubKey().SerializeCompressed()) })
	c.dup("pile")
	c.dup("action")
	c.op("remaining", 2, t.OP_SUB)
	c.num("base", 1020000)
	c.dup("turn")
	c.num("fee", 20000)
	c.op("used", 2, t.OP_MUL)
	c.op("amount", 2, t.OP_SUB)
	c.dup("remaining")
	c.num("zero", 0)
	c.op("cond", 2, t.OP_EQUAL)
	c.choose("outputs", func() {
		c.data("pkstart", []byte{33})
		c.dup("actor")
		c.cat()
		c.data("check", []byte{t.OP_CHECKSIG})
		c.cat()
		c.st[len(c.st)-1] = "dest"
		c.data("count", []byte{1})
		c.out("dest", "amount", 35)
		c.cat()
	}, func() {
		c.dup("turn")
		c.num("limit", 8)
		c.op("cond", 2, t.OP_EQUAL)
		c.choose("outputs", func() {
			c.dup("amount")
			c.num("two", 2)
			c.op("half", 2, t.OP_DIV)
			c.data("pkA", p2pk(keys[0]))
			c.data("pkB", p2pk(keys[1]))
			c.data("count", []byte{2})
			c.out("pkA", "half", 35)
			c.cat()
			c.out("pkB", "half", 35)
			c.cat()
		}, func() {
			c.data("nextstateprefix", append([]byte{32}, make([]byte, 29)...))
			c.dup("remaining")
			c.cat()
			c.num("three", 3)
			c.dup("seat")
			c.op("nextseat", 2, t.OP_SUB)
			c.cat()
			c.dup("turn")
			c.op("nextturn", 1, t.OP_1ADD)
			c.cat()
			c.dup("code")
			c.num("offset", 33)
			c.op("suffix", 2, t.OP_RIGHT)
			c.cat()
			c.op("nextscripthash", 1, t.OP_HASH160)
			c.data("start", []byte{t.OP_HASH160, 20})
			c.b.AddOp(t.OP_SWAP)
			c.cat()
			c.data("end", []byte{t.OP_EQUAL})
			c.cat()
			c.st[len(c.st)-1] = "dest"
			c.data("count", []byte{1})
			c.out("dest", "amount", 23)
			c.cat()
		})
	})
	// Branch-local temporary stack leftovers differ. Compiler choose needs normalize them below (see choose implementation).
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
	b := c.b
	b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256).AddOp(t.OP_DUP).AddInt64(31).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddInt64(31).AddOp(t.OP_RIGHT).AddOp(t.OP_DUP).AddInt64(1).AddInt64(127).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY).AddOp(t.OP_1ADD).AddOp(t.OP_CAT).AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{1}).AddOp(t.OP_CAT).AddData(append([]byte{3}, gx...)).AddInt64(2).AddOp(t.OP_CHECKSIGALTVERIFY)
	c.st = c.st[:len(c.st)-1]
	c.dup("actor")
	c.op("", 1, t.OP_TOALTSTACK)
	c.dup("action")
	c.op("", 1, t.OP_TOALTSTACK)
	for len(c.st) > 1 {
		if len(c.st) > 2 {
			c.op("", 2, t.OP_2DROP)
		} else {
			c.op("", 1, t.OP_DROP)
		}
	}
	b.AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_IF).AddOp(t.OP_DUP).AddOp(t.OP_SIZE).AddOp(t.OP_1SUB).AddOp(t.OP_RIGHT).AddData([]byte{1}).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_CHECKSIG).AddOp(t.OP_ELSE).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_2DROP).AddInt64(2).AddOp(t.OP_CHECKSEQUENCEVERIFY).AddOp(t.OP_DROP).AddOp(t.OP_TRUE).AddOp(t.OP_ENDIF)
	v, e := b.Script()
	must(e)
	return v
}
func redeem(p, s, u byte) []byte {
	v := script(p, s, u, 0)
	for i := 0; i < 3; i++ {
		v = script(p, s, u, len(v))
	}
	return v
}
func prefix(tx *wire.MsgTx) []byte { b, e := tx.BytesPrefix(); must(e); return b }
func witness(tx *wire.MsgTx, r, claim []byte, k *secp256k1.PrivateKey, action int64) {
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
	s, e := sign.RawTxInSignature(tx, 0, r, t.SigHashAll, k.Serialize(), dcrec.STEcdsaSecp256k1)
	must(e)
	tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(s).AddInt64(action).AddData(prefix(tx)).AddData(claim).AddData(r).Script()
	must(e)
}
func prepare(p, s, u byte, a int64) (*wire.MsgTx, []byte) {
	r := redeem(p, s, u)
	tx := wire.NewMsgTx()
	tx.Version = 2
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, int64(1040000-20000*int(u)), nil))
	tx.TxIn[0].Sequence = 0xfffffffe
	if a == 0 {
		tx.TxIn[0].Sequence = 2
	}
	amount := int64(1020000 - 20000*int(u))
	if int64(p) == a {
		tx.AddTxOut(wire.NewTxOut(amount, p2pk(keys[s-1])))
	} else if u == 8 {
		tx.AddTxOut(wire.NewTxOut(amount/2, p2pk(keys[0])))
		tx.AddTxOut(wire.NewTxOut(amount/2, p2pk(keys[1])))
	} else {
		tx.AddTxOut(wire.NewTxOut(amount, p2sh(redeem(p-byte(a), 3-s, u+1))))
	}
	witness(tx, r, r, keys[s-1], a)
	return tx, r
}
func check(label string, tx *wire.MsgTx, r []byte, want bool) {
	vm, e := t.NewEngine(p2sh(r), tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySigPushOnly|t.ScriptVerifyCheckSequenceVerify, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	fmt.Printf("%s redeem%d witness%d tx%d: %v\n", label, len(r), len(tx.TxIn[0].SignatureScript), tx.SerializeSize(), e)
	if (e == nil) != want {
		panic("unexpected")
	}
}

var fixtureDir string

func export(name string, tx *wire.MsgTx, r []byte) {
	if fixtureDir == "" {
		return
	}
	must(os.MkdirAll(fixtureDir, 0700))
	b, e := tx.Bytes()
	must(e)
	must(os.WriteFile(filepath.Join(fixtureDir, name+".tx"), b, 0600))
	must(os.WriteFile(filepath.Join(fixtureDir, name+"-prevout.script"), p2sh(r), 0600))
	if name == "valid" {
		must(os.WriteFile(filepath.Join(fixtureDir, "prevout.script"), p2sh(r), 0600))
	}
}
func main() {
	flag.StringVar(&fixtureDir, "export", "", "directory for synthetic fixtures (optional)")
	flag.StringVar(&checkpointFixtureDir, "checkpoint-export", "", "directory for bounded checkpoint fixtures (optional)")
	flag.Parse()
	runProbe()
	if checkpointFixtureDir != "" {
		runCheckpointProbe()
	}
}
func runProbe() {
	count := 0
	for p := byte(1); p <= 3; p++ {
		for s := byte(1); s <= 2; s++ {
			for u := byte(1); u <= 8; u++ {
				for a := int64(0); a <= 2 && a <= int64(p); a++ {
					tx, r := prepare(p, s, u, a)
					v, e := t.NewEngine(p2sh(r), tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySigPushOnly|t.ScriptVerifyCheckSequenceVerify, 0, nil)
					if e == nil {
						e = v.Execute()
					}
					if e != nil {
						panic(fmt.Sprintf("state %d %d %d move%d: %v", p, s, u, a, e))
					}
					count++
				}
			}
		}
	}
	fmt.Printf("%d positive combinations passed\n", count)
	tx, r := prepare(3, 1, 1, 1)
	check("initial choose1", tx, r, true)
	export("valid", tx, r)
	tx, r = prepare(3, 1, 1, 0)
	witness(tx, r, r, secp256k1.PrivKeyFromBytes([]byte{99}), 0)
	check("skip without active actor", tx, r, true)
	export("timeout", tx, r)
	tx, r = prepare(1, 1, 3, 1)
	check("playerA wins", tx, r, true)
	export("winA", tx, r)
	tx, r = prepare(2, 2, 2, 2)
	check("playerB wins", tx, r, true)
	export("winB", tx, r)
	tx, r = prepare(3, 2, 8, 0)
	witness(tx, r, r, secp256k1.PrivKeyFromBytes([]byte{99}), 0)
	check("draw without either actor", tx, r, true)
	export("draw", tx, r)
	attack := func(label string, f func(*wire.MsgTx, []byte)) {
		tx, r := prepare(3, 1, 1, 1)
		f(tx, r)
		check(label, tx, r, false)
	}
	attack("redirect with fresh actor signature", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2pk(keys[0]); witness(tx, r, r, keys[0], 1) })
	attack("amount with fresh actor signature", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].Value++; witness(tx, r, r, keys[0], 1) })
	attack("wrong actor", func(tx *wire.MsgTx, r []byte) { witness(tx, r, r, keys[1], 1) })
	attack("move3 forbidden", func(tx *wire.MsgTx, r []byte) { witness(tx, r, r, keys[0], 3) })
	attack("negative move forbidden", func(tx *wire.MsgTx, r []byte) { witness(tx, r, r, keys[0], -1) })
	attack("selfloop", func(tx *wire.MsgTx, r []byte) { tx.TxOut[0].PkScript = p2sh(r); witness(tx, r, r, keys[0], 1) })
	attack("expiry", func(tx *wire.MsgTx, r []byte) { tx.Expiry = 1; witness(tx, r, r, keys[0], 1) })
	attack("forged current seat", func(tx *wire.MsgTx, r []byte) { witness(tx, r, redeem(3, 2, 1), keys[0], 1) })
	attack("forged suffix and matching successor", func(tx *wire.MsgTx, r []byte) {
		fake := append([]byte{}, r...)
		fake[len(fake)-1] = t.OP_TRUE
		next := append([]byte{}, fake...)
		next[30] = 2
		next[31] = 2
		next[32] = 2
		tx.TxOut[0].PkScript = p2sh(next)
		witness(tx, r, fake, keys[0], 1)
	})
	tx, r = prepare(1, 1, 1, 1)
	witness(tx, r, r, keys[0], 2)
	check("remove more than pile", tx, r, false)
	tx, r = prepare(3, 1, 1, 0)
	tx.TxIn[0].Sequence = 1
	witness(tx, r, r, keys[0], 0)
	check("early skip", tx, r, false)
	tx, r = prepare(3, 1, 1, 0)
	tx.TxIn[0].Sequence = 0x80000002
	witness(tx, r, r, keys[0], 0)
	check("CSV disabled skip", tx, r, false)
	tx, r = prepare(3, 1, 1, 0)
	tx.TxOut[0].PkScript = p2pk(keys[1])
	witness(tx, r, r, keys[1], 0)
	check("accuser payout instead of skip", tx, r, false)
	tx, r = prepare(3, 2, 8, 0)
	tx.TxOut[0].Value++
	tx.TxOut[1].Value--
	witness(tx, r, r, keys[1], 0)
	check("unequal draw", tx, r, false)
}
