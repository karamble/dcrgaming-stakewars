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
	"os"
	"path/filepath"
	"sync"
)

func fixture(claim int64, corrupt int) (*wire.MsgTx, []byte, []byte) {
	root, proof, value := memoryFixture()
	if corrupt >= 0 {
		proof[corrupt][0] ^= 1
	}
	gx, _ := hex.DecodeString("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
	q := append([]byte{3}, gx...)
	dest, _ := t.NewScriptBuilder().AddOp(t.OP_DUP).AddOp(t.OP_HASH160).AddData(bytes.Repeat([]byte{7}, 20)).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_CHECKSIG).Script()
	out := append([]byte{0x10, 0x27, 0, 0, 0, 0, 0, 0, 0, 0, 25}, dest...)
	b := t.NewScriptBuilder()
	b.AddOp(t.OP_DUP).AddInt64(1).AddInt64(101).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY)
	b.AddOp(t.OP_DUP).AddOp(t.OP_TOALTSTACK)
	// Domain-separated leaf: 0x00 followed by the canonical one-byte value.
	b.AddData([]byte{0, 0}).AddInt64(1).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	for level := 0; level < depth; level++ {
		b.AddOp(t.OP_SWAP).AddOp(t.OP_SIZE).AddInt64(32).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_SWAP)
		if (address>>level)&1 == 0 {
			b.AddOp(t.OP_SWAP)
		}
		b.AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	}
	b.AddData(root[:]).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_FROMALTSTACK).AddInt64(7).AddOp(t.OP_ADD).AddInt64(claim).AddOp(t.OP_NUMNOTEQUAL).AddOp(t.OP_VERIFY)
	b.AddOp(t.OP_SIZE).AddInt64(91).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddInt64(87).AddOp(t.OP_RIGHT).AddData([]byte{0, 0, 0, 0}).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddInt64(46).AddInt64(42).AddOp(t.OP_SUBSTR).AddData([]byte{255, 255, 255, 255}).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddInt64(83).AddInt64(47).AddOp(t.OP_SUBSTR).AddData(out).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_BLAKE256).AddData([]byte{1, 0, 0, 0}).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	b.AddOp(t.OP_DUP).AddInt64(31).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddInt64(31).AddOp(t.OP_RIGHT)
	b.AddOp(t.OP_DUP).AddInt64(1).AddInt64(127).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY).AddOp(t.OP_1ADD).AddOp(t.OP_CAT)
	b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{1}).AddOp(t.OP_CAT).AddData(q).AddInt64(2).AddOp(t.OP_CHECKSIGALT)
	redeem, err := b.Script()
	must(err)
	bh := blake256.Sum256(redeem)
	rh := ripemd160.New()
	rh.Write(bh[:])
	pk, _ := t.NewScriptBuilder().AddOp(t.OP_HASH160).AddData(rh.Sum(nil)).AddOp(t.OP_EQUAL).Script()
	tx := wire.NewMsgTx()
	tx.Version = 2
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, 30000, nil))
	tx.AddTxOut(wire.NewTxOut(10000, dest))
	var prefix, witness []byte
	for {
		tx.LockTime++
		var p bytes.Buffer
		binary.Write(&p, binary.LittleEndian, uint32(2|1<<16))
		p.WriteByte(1)
		p.Write(make([]byte, 37))
		binary.Write(&p, binary.LittleEndian, uint32(0xffffffff))
		p.WriteByte(1)
		p.Write(out)
		binary.Write(&p, binary.LittleEndian, tx.LockTime)
		binary.Write(&p, binary.LittleEndian, tx.Expiry)
		prefix = p.Bytes()
		var w bytes.Buffer
		binary.Write(&w, binary.LittleEndian, uint32(2|3<<16))
		w.WriteByte(1)
		wire.WriteVarInt(&w, 0, uint64(len(redeem)))
		w.Write(redeem)
		wh := blake256.Sum256(w.Bytes())
		witness = wh[:]
		ph := blake256.Sum256(prefix)
		pre := append([]byte{1, 0, 0, 0}, ph[:]...)
		pre = append(pre, witness...)
		mh := blake256.Sum256(pre)
		live, err := t.CalcSignatureHash(redeem, t.SigHashAll, tx, 0, nil)
		must(err)
		if !bytes.Equal(live, mh[:]) {
			panic("sighash mismatch")
		}
		e := blake256.Sum256(append(append([]byte{}, gx...), mh[:]...))
		if e[31] >= 1 && e[31] < 127 {
			break
		}
	}
	sb := t.NewScriptBuilder().AddData(witness).AddData(prefix)
	for level := depth - 1; level >= 0; level-- {
		sb.AddData(proof[level][:])
	}
	sig, err := sb.AddInt64(int64(value)).AddData(redeem).Script()
	must(err)
	tx.TxIn[0].SignatureScript = sig
	return tx, pk, redeem
}
func check(tx *wire.MsgTx, pk []byte) error {
	flags := t.ScriptVerifyCleanStack | t.ScriptVerifySigPushOnly | t.ScriptDiscourageUpgradableNops | t.ScriptVerifyCheckLockTimeVerify | t.ScriptVerifyCheckSequenceVerify | t.ScriptVerifySHA256 | t.ScriptVerifyTreasury
	vm, err := t.NewEngine(pk, tx, 0, flags, 0, nil)
	if err == nil {
		err = vm.Execute()
	}
	return err
}
func main() {
	export := flag.String("export", "", "directory for synthetic transaction fixtures (optional)")
	flag.Parse()
	tx, pk, r := fixture(108, -1)
	must(check(tx, pk))
	fmt.Printf("PASS redeem=%d sigscript=%d grind=%d sigops=%d\n", len(r), len(tx.TxIn[0].SignatureScript), tx.LockTime, t.GetPreciseSigOpCount(tx.TxIn[0].SignatureScript, pk, true))
	b, e := tx.Bytes()
	must(e)
	if *export != "" {
		must(os.MkdirAll(*export, 0700))
		must(os.WriteFile(filepath.Join(*export, "valid.tx"), b, 0600))
		must(os.WriteFile(filepath.Join(*export, "prevout.script"), pk, 0600))
	}
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}

const depth = 19
const address = 21917

// Construct the complete memory, not just an arbitrary root/path pair.
var memoryOnce sync.Once
var memoryRoot [32]byte
var memoryPath [depth][32]byte

func memoryFixture() ([32]byte, [depth][32]byte, byte) {
	memoryOnce.Do(func() { memoryRoot, memoryPath, _ = buildMemory() })
	return memoryRoot, memoryPath, byte(address%100 + 1)
}
func buildMemory() ([32]byte, [depth][32]byte, byte) {
	nodes := make([][32]byte, 1<<depth)
	for i := range nodes {
		nodes[i] = blake256.Sum256([]byte{0, byte(i%100 + 1)})
	}
	var proof [depth][32]byte
	index := address
	for level := 0; level < depth; level++ {
		proof[level] = nodes[index^1]
		for j := 0; j < len(nodes)/2; j++ {
			var pair [64]byte
			copy(pair[:32], nodes[2*j][:])
			copy(pair[32:], nodes[2*j+1][:])
			nodes[j] = blake256.Sum256(pair[:])
		}
		nodes = nodes[:len(nodes)/2]
		index >>= 1
	}
	return nodes[0], proof, byte(address%100 + 1)
}
