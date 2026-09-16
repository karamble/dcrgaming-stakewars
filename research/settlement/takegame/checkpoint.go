package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/decred/dcrd/crypto/blake256"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"os"
	"path/filepath"
)

var secrets [2][4][32]byte

func init() {
	for p := 0; p < 2; p++ {
		for i := 0; i < 4; i++ {
			secrets[p][i] = blake256.Sum256([]byte(fmt.Sprintf("checkpoint-only-toy/session42/rules-take3/rosterAB/player%d/snapshot%d", p, i)))
		}
	}
}
func dictionary() []byte {
	var d []byte
	for _, r := range [][]byte{redeem(3, 1, 1), redeem(2, 2, 2), redeem(1, 1, 3), p2pk(keys[0])} {
		d = append(d, p2sh(r)...)
	}
	return d
}
func publicDictionary() []byte {
	var d []byte
	for p := 0; p < 2; p++ {
		for _, s := range secrets[p] {
			h := blake256.Sum256(s[:])
			d = append(d, h[:]...)
		}
	}
	return append(d, dictionary()...)
}
func checkpointScript(cur byte, n int) []byte {
	c := &compiler{t.NewScriptBuilder(), []string{"dict", "certA", "certB", "target", "prefix", "code"}}
	st := make([]byte, 32)
	st[31] = cur
	c.data("state", st)
	c.num("offset", 31)
	c.op("cur", 2, t.OP_RIGHT)
	c.dup("dict")
	c.op("dictHash", 1, t.OP_BLAKE256)
	h := blake256.Sum256(publicDictionary())
	c.data("boundHash", h[:])
	c.op("", 2, t.OP_EQUALVERIFY)
	c.dup("target")
	c.num("min", 0)
	c.num("max", 6)
	c.op("", 3, t.OP_WITHIN, t.OP_VERIFY)
	c.dup("target")
	c.num("zero", 0)
	c.op("update", 2, t.OP_GREATERTHAN)
	c.choose("dest", func() {
		c.dup("target")
		c.dup("cur")
		c.op("", 2, t.OP_GREATERTHAN, t.OP_VERIFY)
		c.dup("target")
		c.num("min", 2)
		c.num("max", 6)
		c.op("", 3, t.OP_WITHIN, t.OP_VERIFY)
		for p := 0; p < 2; p++ {
			name := "certA"
			if p == 1 {
				name = "certB"
			}
			c.dup(name)
			c.op("h", 1, t.OP_BLAKE256)
			c.dup("dict")
			c.dup("target")
			c.num("base", 2)
			c.op("idx", 2, t.OP_SUB)
			c.num("width", 32)
			c.op("start", 2, t.OP_MUL)
			if p == 1 {
				c.num("playeroffset", 128)
				c.op("start", 2, t.OP_ADD)
			}
			c.dup("start")
			c.num("width", 32)
			c.op("end", 2, t.OP_ADD)
			c.b.AddOp(t.OP_SWAP)
			c.op("expected", 3, t.OP_SUBSTR)
			c.op("", 2, t.OP_EQUALVERIFY)
		}
		c.data("newprefix", append([]byte{32}, make([]byte, 31)...))
		c.dup("target")
		c.cat()
		c.dup("code")
		c.num("off", 33)
		c.op("suffix", 2, t.OP_RIGHT)
		c.cat()
		c.op("h", 1, t.OP_HASH160)
		c.data("start", []byte{t.OP_HASH160, 20})
		c.b.AddOp(t.OP_SWAP)
		c.cat()
		c.data("end", []byte{t.OP_EQUAL})
		c.cat()
	}, func() {
		c.dup("cur")
		c.num("genesis", 1)
		c.op("", 2, t.OP_GREATERTHAN, t.OP_VERIFY)
		c.num("delay", 2)
		c.op("", 1, t.OP_CHECKSEQUENCEVERIFY, t.OP_DROP)
		c.dup("dict")
		c.dup("cur")
		c.num("base", 2)
		c.op("idx", 2, t.OP_SUB)
		c.num("width", 23)
		c.op("start", 2, t.OP_MUL)
		c.num("destoffset", 256)
		c.op("start", 2, t.OP_ADD)
		c.dup("start")
		c.num("width", 23)
		c.op("end", 2, t.OP_ADD)
		c.b.AddOp(t.OP_SWAP)
		c.op("pk", 3, t.OP_SUBSTR)
	})
	c.dup("target")
	c.num("zero", 0)
	c.op("update", 2, t.OP_GREATERTHAN)
	c.choose("feeIndex", func() { c.dup("target") }, func() { c.dup("cur"); c.op("next", 1, t.OP_1ADD) })
	c.num("base", 1080000)
	c.dup("feeIndex")
	c.num("fee", 20000)
	c.op("used", 2, t.OP_MUL)
	c.op("amount", 2, t.OP_SUB)
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
	c.op("actual", 3, t.OP_SUBSTR)
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
	c.b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256).AddOp(t.OP_DUP).AddInt64(31).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddInt64(31).AddOp(t.OP_RIGHT).AddOp(t.OP_DUP).AddInt64(1).AddInt64(127).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY).AddOp(t.OP_1ADD).AddOp(t.OP_CAT).AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{1}).AddOp(t.OP_CAT).AddData(append([]byte{3}, gx...)).AddInt64(2).AddOp(t.OP_CHECKSIGALTVERIFY)
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
func checkpoint(cur byte) []byte {
	v := checkpointScript(cur, 0)
	for i := 0; i < 3; i++ {
		v = checkpointScript(cur, len(v))
	}
	return v
}
func checkpointAuthorize(tx *wire.MsgTx, r, claimed []byte, target byte, a, b []byte) {
	tx.LockTime = 0
	for {
		tx.LockTime++
		if tx.LockTime > 255 {
			panic("bounded nonce search exhausted")
		}
		h, e := t.CalcSignatureHash(r, t.SigHashAll, tx, 0, nil)
		must(e)
		x := blake256.Sum256(append(append([]byte{}, gx...), h...))
		if x[31] >= 1 && x[31] < 127 {
			break
		}
	}
	var e error
	tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(publicDictionary()).AddData(a).AddData(b).AddInt64(int64(target)).AddData(prefix(tx)).AddData(claimed).AddData(r).Script()
	must(e)
}
func checkpointPrepare(cur, target byte) (*wire.MsgTx, []byte) {
	r := checkpoint(cur)
	tx := wire.NewMsgTx()
	tx.Version = 2
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, 1080000-int64(cur)*20000, nil))
	tx.TxIn[0].Sequence = 0xfffffffe
	var a, b []byte
	var pk []byte
	idx := target
	if target == 0 {
		idx = cur + 1
		tx.TxIn[0].Sequence = 2
		d := dictionary()
		pk = d[(cur-2)*23 : (cur-1)*23]
	} else {
		pk = p2sh(checkpoint(target))
		a = secrets[0][target-2][:]
		b = secrets[1][target-2][:]
	}
	tx.AddTxOut(wire.NewTxOut(1080000-int64(idx)*20000, pk))
	checkpointAuthorize(tx, r, r, target, a, b)
	return tx, r
}
func checkpointVerify(tx *wire.MsgTx, r []byte) error {
	vm, e := t.NewEngine(p2sh(r), tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySigPushOnly|t.ScriptVerifyCheckSequenceVerify|t.ScriptDiscourageUpgradableNops, 0, nil)
	if e == nil {
		e = vm.Execute()
	}
	return e
}

var checkpointFixtureDir string

func checkpointExport(name string, tx *wire.MsgTx, r []byte) {
	if checkpointFixtureDir == "" {
		return
	}
	must(os.MkdirAll(checkpointFixtureDir, 0700))
	b, e := tx.Bytes()
	must(e)
	must(os.WriteFile(filepath.Join(checkpointFixtureDir, name+".tx"), b, 0600))
	must(os.WriteFile(filepath.Join(checkpointFixtureDir, name+"-prevout.script"), p2sh(r), 0600))
}
func linkedFixtures() {
	claim, cr := checkpointPrepare(1, 3)
	checkpointExport("claim", claim, cr)
	update, ur := checkpointPrepare(3, 4)
	update.TxIn[0].PreviousOutPoint.Hash = claim.TxHash()
	checkpointAuthorize(update, ur, ur, 4, secrets[0][2][:], secrets[1][2][:])
	must(checkpointVerify(update, ur))
	checkpointExport("update", update, ur)
	stale, sr := checkpointPrepare(3, 0)
	stale.TxIn[0].PreviousOutPoint.Hash = claim.TxHash()
	checkpointAuthorize(stale, sr, sr, 0, nil, nil)
	must(checkpointVerify(stale, sr))
	checkpointExport("stale-exit", stale, sr)
	latest, lr := checkpointPrepare(4, 0)
	latest.TxIn[0].PreviousOutPoint.Hash = update.TxHash()
	checkpointAuthorize(latest, lr, lr, 0, nil, nil)
	must(checkpointVerify(latest, lr))
	checkpointExport("latest-exit", latest, lr)
	final, fr := prepare(1, 1, 3, 1)
	final.TxIn[0].PreviousOutPoint.Hash = latest.TxHash()
	witness(final, fr, fr, keys[0], 1)
	must(checkpointVerify(final, fr))
	checkpointExport("game-final", final, fr)
	alternative, ar := prepare(2, 2, 2, 2)
	alternative.TxIn[0].PreviousOutPoint.Hash = stale.TxHash()
	witness(alternative, ar, ar, keys[1], 2)
	must(checkpointVerify(alternative, ar))
	checkpointExport("stale-alternative", alternative, ar)
}
func runCheckpointProbe() {
	for _, pair := range [][2]byte{{1, 2}, {1, 3}, {1, 4}, {2, 3}, {3, 4}, {4, 5}, {2, 0}, {3, 0}, {4, 0}, {5, 0}} {
		tx, r := checkpointPrepare(pair[0], pair[1])
		e := checkpointVerify(tx, r)
		fmt.Printf("%d->%d redeem%d witness%d tx%d: %v\n", pair[0], pair[1], len(r), len(tx.TxIn[0].SignatureScript), tx.SerializeSize(), e)
		must(e)
	}
	linkedFixtures()
}
