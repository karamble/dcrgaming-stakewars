package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/decred/dcrd/dcrec"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	txscript "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/sign"
	"github.com/decred/dcrd/wire"
)

func actor(state byte) *secp256k1.PrivateKey {
	if state == 2 {
		return keyB
	}
	return key
}

func TestLinkedRecursiveTransitions(t *testing.T) {
	var parent *wire.MsgTx
	for state := byte(1); state <= 3; state++ {
		tx, r := prepare(state)
		if parent != nil {
			if !bytes.Equal(parent.TxOut[0].PkScript, p2sh(r)) {
				t.Fatal("successor script mismatch")
			}
			tx.TxIn[0].PreviousOutPoint.Hash = parent.TxHash()
			tx.TxIn[0].ValueIn = parent.TxOut[0].Value
			witness(tx, r, r, actor(state))
		}
		if err := verify(tx, r); err != nil {
			t.Fatal(err)
		}
		if len(tx.TxIn[0].SignatureScript) > 1650 {
			t.Fatal("exceeds standard signature script size")
		}
		if tx.TxIn[0].ValueIn-tx.TxOut[0].Value != 20000 {
			t.Fatal("incorrect fee for correctly funded state")
		}
		parent = tx
	}
	if !bytes.Equal(parent.TxOut[0].PkScript, payout()) {
		t.Fatal("wrong terminal recipient")
	}
}

func TestAuthorizedActorCannotEscapeCovenant(t *testing.T) {
	mutations := []struct {
		name   string
		change func(*wire.MsgTx)
	}{
		{"amount", func(tx *wire.MsgTx) { tx.TxOut[0].Value++ }},
		{"destination", func(tx *wire.MsgTx) { tx.TxOut[0].PkScript = []byte{txscript.OP_TRUE} }},
		{"expiry", func(tx *wire.MsgTx) { tx.Expiry = 1500000 }},
		{"output version", func(tx *wire.MsgTx) { tx.TxOut[0].Version = 1 }},
		{"transaction version", func(tx *wire.MsgTx) { tx.Version = 1 }},
		{"extra output", func(tx *wire.MsgTx) { tx.AddTxOut(wire.NewTxOut(1000, payout())) }},
		{"extra input", func(tx *wire.MsgTx) { tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: 1}, 1000, nil)) }},
	}
	for state := byte(1); state <= 3; state++ {
		for _, m := range mutations {
			t.Run(fmt.Sprintf("state%d/%s", state, m.name), func(t *testing.T) {
				tx, r := prepare(state)
				m.change(tx)
				// Fresh authorization and actual mutated prefix: this tests the
				// covenant, not rejection of an obsolete ordinary signature.
				witness(tx, r, r, actor(state))
				if verify(tx, r) == nil {
					t.Fatal("malicious authorized spend accepted")
				}
			})
		}
	}
}

func TestAuthenticatedCurrentScript(t *testing.T) {
	for _, change := range []string{"state", "executable suffix"} {
		t.Run(change, func(t *testing.T) {
			tx, r := prepare(1)
			claimed := append([]byte(nil), r...)
			if change == "state" {
				claimed = redeem(2)
			} else {
				claimed[len(claimed)-1] = txscript.OP_TRUE
			}
			witness(tx, r, claimed, key)
			if verify(tx, r) == nil {
				t.Fatal("forged live script accepted")
			}
		})
	}
	tx, r := prepare(1)
	tx.TxOut[0].PkScript = p2sh(r)
	witness(tx, r, r, key)
	if verify(tx, r) == nil {
		t.Fatal("state self-loop accepted")
	}
}

func TestSyntheticKeyDoesNotAuthorizeMove(t *testing.T) {
	secret, err := hex.DecodeString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140")
	if err != nil {
		t.Fatal(err)
	}
	tx, r := prepare(1)
	witness(tx, r, r, secp256k1.PrivKeyFromBytes(secret))
	if verify(tx, r) == nil {
		t.Fatal("public synthetic scalar authorized player action")
	}
}

func TestActorMustSignAll(t *testing.T) {
	for _, hashType := range []txscript.SigHashType{txscript.SigHashNone, txscript.SigHashSingle, txscript.SigHashAll | txscript.SigHashAnyOneCanPay} {
		t.Run(fmt.Sprint(hashType), func(t *testing.T) {
			tx, r := prepare(1)
			sig, err := sign.RawTxInSignature(tx, 0, r, hashType, key.Serialize(), dcrec.STEcdsaSecp256k1)
			if err != nil {
				t.Fatal(err)
			}
			tx.TxIn[0].SignatureScript, err = txscript.NewScriptBuilder().AddData(sig).AddInt64(1).AddData(prefix(tx)).AddData(r).AddData(r).Script()
			if err != nil {
				t.Fatal(err)
			}
			if verify(tx, r) == nil {
				t.Fatal("weaker actor authorization accepted")
			}
		})
	}
}

func TestTimeoutNeedsNoActorButRequiresCSVEncoding(t *testing.T) {
	for state := byte(1); state <= 3; state++ {
		tx, r := timeout(state)
		if err := verify(tx, r); err != nil {
			t.Fatal(err)
		}
		tx.TxIn[0].Sequence = 1
		witnessMode(tx, r, r, key, 0)
		if verify(tx, r) == nil {
			t.Fatal("insufficient CSV sequence accepted")
		}
	}
	// Actual input age is checked in the separate stock dcrd mempool harness.
}
