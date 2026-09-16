package main

import (
	"bytes"
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

func TestFifteenLinkedChainSteps(tst *testing.T) {
	opening := initialOpening()
	var previous *wire.MsgTx
	for step := 0; step < 15; step++ {
		tx, r, next := fixture(opening)
		if previous != nil {
			tx.TxIn[0].PreviousOutPoint.Hash = previous.TxHash()
			wotsWitness(tx, r, opening)
			if !bytes.Equal(previous.TxOut[0].PkScript, p2sh(r)) || previous.TxOut[0].Value != tx.TxIn[0].ValueIn {
				tst.Fatal("broken pot lineage")
			}
		}
		if err := check(tx, r); err != nil {
			tst.Fatalf("step%d: %v", step, err)
		}
		if len(tx.TxIn[0].SignatureScript) > 1650 || countOps(r) > 255 {
			tst.Fatal("resource limit")
		}
		if !bytes.Equal(opening[:91], next[:91]) || !bytes.Equal(opening[92:96], next[92:96]) || next[91] != opening[91]+1 {
			tst.Fatal("context changed or skipped position")
		}
		if tx.TxIn[0].ValueIn-tx.TxOut[0].Value != 20000 {
			tst.Fatal("fee continuity")
		}
		previous = tx
		opening = next
	}
	// This primitive deliberately ends here: no endpoint authentication, next
	// WOTS chain, certificate completion, abort, or release branch is implemented.
	tx, r, _ := fixture(opening)
	if check(tx, r) == nil {
		tst.Fatal("unimplemented terminal phase executed")
	}
}
func TestFragmentAttacks(tst *testing.T) {
	for name, edit := range map[string]func([]byte){
		"statement": func(st []byte) { st[0] ^= 1 }, "public seed": func(st []byte) { st[32] ^= 1 },
		"signer slot": func(st []byte) { st[80] ^= 1 }, "chain position": func(st []byte) { st[87] ^= 1 },
		"skip step": func(st []byte) { st[91]++ }, "chain value": func(st []byte) { st[96] ^= 1 },
		"pot amount": func(st []byte) { setAmount(st, amount(st)+1) },
	} {
		tst.Run(name, func(tst *testing.T) {
			st := initialOpening()
			tx, r, next := fixture(st)
			edit(next)
			tx.TxOut[0].PkScript = p2sh(wotsRedeem(next))
			wotsWitness(tx, r, st)
			if check(tx, r) == nil {
				tst.Fatal("mutated successor accepted")
			}
		})
	}
	st := initialOpening()
	tx, r, _ := fixture(st)
	forged := append([]byte{}, st...)
	forged[0] ^= 1
	wotsWitness(tx, r, forged)
	if check(tx, r) == nil {
		tst.Fatal("forged opening accepted")
	}
	tx, r, _ = fixture(st)
	tx.TxOut[0].Value++
	wotsWitness(tx, r, st)
	if check(tx, r) == nil {
		tst.Fatal("wrong amount accepted")
	}
	tx, r, _ = fixture(st)
	tx.Expiry = 1
	wotsWitness(tx, r, st)
	if check(tx, r) == nil {
		tst.Fatal("expiry accepted")
	}
	tx, r, _ = fixture(st)
	tx.TxOut[0].PkScript = p2sh(r)
	wotsWitness(tx, r, st)
	if check(tx, r) == nil {
		tst.Fatal("selfloop accepted")
	}
	tx, r, next := fixture(st)
	later, wrongRedeem, _ := fixture(next)
	later.TxIn[0].PreviousOutPoint = tx.TxIn[0].PreviousOutPoint
	wotsWitness(later, wrongRedeem, next)
	vm, err := t.NewEngine(p2sh(r), later, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySHA256, 0, nil)
	if err == nil {
		err = vm.Execute()
	}
	if err == nil {
		tst.Fatal("reordered fragment accepted")
	}
}
func TestRejectHighHashStepAddress(tst *testing.T) {
	for _, i := range []int{88, 89, 90} {
		st := initialOpening()
		st[i] = 1
		tx, r, _ := fixture(st)
		if check(tx, r) == nil {
			tst.Fatalf("high address byte%d accepted", i)
		}
	}
}
