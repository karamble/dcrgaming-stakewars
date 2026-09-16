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
var keyB = secp256k1.PrivKeyFromBytes([]byte{44})
var key = secp256k1.PrivKeyFromBytes([]byte{42})

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func p2sh(s []byte) []byte {
	bh := blake256.Sum256(s)
	r := ripemd160.New()
	r.Write(bh[:])
	v, e := t.NewScriptBuilder().AddOp(t.OP_HASH160).AddData(r.Sum(nil)).AddOp(t.OP_EQUAL).Script()
	must(e)
	return v
}
func payoutFor(k *secp256k1.PrivateKey) []byte {
	v, e := t.NewScriptBuilder().AddData(k.PubKey().SerializeCompressed()).AddOp(t.OP_CHECKSIG).Script()
	must(e)
	return v
}
func payout() []byte { return payoutFor(key) }
func script(state byte, n int) []byte {
	b := t.NewScriptBuilder()
	st := make([]byte, 32)
	st[31] = state
	// witness stack: actionSig, prefix, suppliedCurrentRedeem; executable state push.
	b.AddData(st).AddInt64(31).AddOp(t.OP_RIGHT).AddOp(t.OP_DUP).AddInt64(1).AddInt64(4).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY)
	b.AddOp(t.OP_DUP).AddInt64(2).AddOp(t.OP_EQUAL).AddOp(t.OP_IF).AddData(keyB.PubKey().SerializeCompressed()).AddOp(t.OP_ELSE).AddData(key.PubKey().SerializeCompressed()).AddOp(t.OP_ENDIF).AddOp(t.OP_TOALTSTACK)
	b.AddOp(t.OP_DUP).AddOp(t.OP_TOALTSTACK).AddInt64(3).AddOp(t.OP_PICK).AddOp(t.OP_IF).AddInt64(3).AddOp(t.OP_EQUAL).AddOp(t.OP_IF)
	b.AddData(payout())
	b.AddOp(t.OP_ELSE)
	// Same executable suffix, next state only. supplied code authenticated by sighash later.
	b.AddOp(t.OP_DUP).AddInt64(33).AddOp(t.OP_RIGHT)
	b.AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_DUP).AddOp(t.OP_TOALTSTACK).AddOp(t.OP_1ADD)
	b.AddData(append([]byte{32}, make([]byte, 31)...)).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_HASH160)
	b.AddData([]byte{t.OP_HASH160, 20}).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{t.OP_EQUAL}).AddOp(t.OP_CAT)
	b.AddOp(t.OP_ENDIF)
	b.AddOp(t.OP_ELSE).AddInt64(2).AddOp(t.OP_EQUAL).AddOp(t.OP_IF).AddData(payoutFor(key)).AddOp(t.OP_ELSE).AddData(payoutFor(keyB)).AddOp(t.OP_ENDIF).AddOp(t.OP_ENDIF)
	// Compute output serialization, pay current pot less fixed1000 fee each transition.
	b.AddOp(t.OP_SIZE).AddOp(t.OP_SWAP).AddOp(t.OP_TOALTSTACK)                                        // code,len;alt count,pk
	b.AddData([]byte{0, 0}).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_CAT) // code,ver||len||pk
	b.AddOp(t.OP_FROMALTSTACK).AddInt64(20000).AddOp(t.OP_MUL).AddInt64(220000).AddOp(t.OP_SWAP).AddOp(t.OP_SUB).AddData(make([]byte, 5)).AddOp(t.OP_CAT).AddOp(t.OP_SWAP).AddOp(t.OP_CAT)
	b.AddOp(t.OP_TOALTSTACK) // actionSig,prefix,code;alt expectedout
	b.AddOp(t.OP_OVER).AddOp(t.OP_DUP).AddInt64(5).AddOp(t.OP_LEFT).AddData([]byte{2, 0, 1, 0, 1}).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddOp(t.OP_SIZE).AddInt64(7).AddOp(t.OP_SUB).AddOp(t.OP_RIGHT).AddData(make([]byte, 7)).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddInt64(47).AddInt64(46).AddOp(t.OP_SUBSTR).AddData([]byte{1}).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_DUP).AddOp(t.OP_SIZE).AddInt64(8).AddOp(t.OP_SUB).AddInt64(47).AddOp(t.OP_SUBSTR).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_BLAKE256).AddOp(t.OP_TOALTSTACK) // actionSig,prefix,code
	var wh bytes.Buffer
	binary.Write(&wh, binary.LittleEndian, uint32(2|3<<16))
	wh.WriteByte(1)
	wire.WriteVarInt(&wh, 0, uint64(n))
	b.AddData(wh.Bytes()).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	b.AddOp(t.OP_FROMALTSTACK).AddData([]byte{1, 0, 0, 0}).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_BLAKE256)
	b.AddOp(t.OP_DUP).AddInt64(31).AddOp(t.OP_LEFT).AddOp(t.OP_SWAP).AddInt64(31).AddOp(t.OP_RIGHT)
	b.AddOp(t.OP_DUP).AddInt64(1).AddInt64(127).AddOp(t.OP_WITHIN).AddOp(t.OP_VERIFY).AddOp(t.OP_1ADD).AddOp(t.OP_CAT)
	b.AddData(gx).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData([]byte{1}).AddOp(t.OP_CAT).AddData(append([]byte{3}, gx...)).AddInt64(2).AddOp(t.OP_CHECKSIGALTVERIFY)
	b.AddOp(t.OP_DROP).AddOp(t.OP_IF).AddOp(t.OP_DUP).AddOp(t.OP_SIZE).AddOp(t.OP_1SUB).AddOp(t.OP_RIGHT).AddData([]byte{1}).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_CHECKSIG).AddOp(t.OP_ELSE).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_2DROP).AddInt64(2).AddOp(t.OP_CHECKSEQUENCEVERIFY).AddOp(t.OP_DROP).AddOp(t.OP_TRUE).AddOp(t.OP_ENDIF)
	v, e := b.Script()
	must(e)
	return v
}
func redeem(s byte) []byte {
	v := script(s, 0)
	for i := 0; i < 3; i++ {
		v = script(s, len(v))
	}
	return v
}
func prefix(tx *wire.MsgTx) []byte {
	v, e := tx.BytesPrefix()
	must(e)
	return v
}
func witness(tx *wire.MsgTx, r, claimed []byte, k *secp256k1.PrivateKey) {
	witnessMode(tx, r, claimed, k, 1)
}
func witnessMode(tx *wire.MsgTx, r, claimed []byte, k *secp256k1.PrivateKey, mode int64) {
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
	tx.TxIn[0].SignatureScript, e = t.NewScriptBuilder().AddData(s).AddInt64(mode).AddData(prefix(tx)).AddData(claimed).AddData(r).Script()
	must(e)
}
func prepare(state byte) (*wire.MsgTx, []byte) {
	r := redeem(state)
	tx := wire.NewMsgTx()
	tx.Version = 2
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, int64(240000-20000*int(state)), nil))
	tx.TxIn[0].Sequence = 0xfffffffe
	out := payout()
	if state < 3 {
		out = p2sh(redeem(state + 1))
	}
	tx.AddTxOut(wire.NewTxOut(int64(220000-20000*int(state)), out))
	k := key
	if state == 2 {
		k = keyB
	}
	witness(tx, r, r, k)
	return tx, r
}

// verify checks Script only; transaction input age is tested separately in dcrd.
func verify(tx *wire.MsgTx, r []byte) error {
	flags := t.ScriptVerifyCleanStack | t.ScriptVerifySigPushOnly | t.ScriptVerifyCheckSequenceVerify | t.ScriptVerifyCheckLockTimeVerify | t.ScriptVerifySHA256 | t.ScriptVerifyTreasury | t.ScriptDiscourageUpgradableNops
	v, e := t.NewEngine(p2sh(r), tx, 0, flags, 0, nil)
	if e == nil {
		e = v.Execute()
	}
	return e
}
func timeout(state byte) (*wire.MsgTx, []byte) {
	tx, r := prepare(state)
	tx.TxIn[0].Sequence = 2
	dest := keyB
	if state == 2 {
		dest = key
	}
	tx.TxOut[0].PkScript = payoutFor(dest)
	// An unrelated key signs the ignored placeholder; no actor consent is needed.
	witnessMode(tx, r, r, secp256k1.PrivKeyFromBytes([]byte{99}), 0)
	return tx, r
}
func main() {
	export := flag.String("export", "", "directory for synthetic transaction fixtures (optional)")
	flag.Parse()
	for state := byte(1); state <= 3; state++ {
		tx, r := prepare(state)
		must(verify(tx, r))
		fmt.Printf("state %d: redeem=%d signature-script=%d transaction=%d fee=20000 atoms\n", state, len(r), len(tx.TxIn[0].SignatureScript), tx.SerializeSize())
		timed, tr := timeout(state)
		must(verify(timed, tr))
		if *export != "" && state == 2 {
			must(os.MkdirAll(*export, 0700))
			for name, fixture := range map[string]*wire.MsgTx{"valid.tx": tx, "timeout.tx": timed} {
				raw, e := fixture.Bytes()
				must(e)
				must(os.WriteFile(filepath.Join(*export, name), raw, 0600))
			}
			must(os.WriteFile(filepath.Join(*export, "prevout.script"), p2sh(r), 0600))
			must(os.WriteFile(filepath.Join(*export, "timeout-prevout.script"), p2sh(r), 0600))
		}
	}
}
