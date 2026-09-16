package main

import (
	t "github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
	"testing"
)

// This is an executable limitation, not a security guarantee: two legally
// authorized actions can conflict before either is confirmed. A later signed
// transcript does not revoke an earlier actor's ability to sign an alternative.
func TestOffchainAcceptedMoveCanBeReplacedBeforeConfirmation(tst *testing.T) {
	first, firstScript := prepare(3, 1, 1, 1)
	bLose, bScript := prepare(2, 2, 2, 1)
	bLose.TxIn[0].PreviousOutPoint.Hash = first.TxHash()
	witness(bLose, bScript, bScript, keys[1], 1)
	aWin, aScript := prepare(1, 1, 3, 1)
	aWin.TxIn[0].PreviousOutPoint.Hash = bLose.TxHash()
	witness(aWin, aScript, aScript, keys[0], 1)
	bAlternative, _ := prepare(2, 2, 2, 2)
	bAlternative.TxIn[0].PreviousOutPoint.Hash = first.TxHash()
	witness(bAlternative, bScript, bScript, keys[1], 2)
	for name, entry := range map[string]struct {
		tx     *wire.MsgTx
		script []byte
	}{
		"A move": {first, firstScript}, "B accepted losing move": {bLose, bScript},
		"A accepted win": {aWin, aScript}, "B conflicting winning move": {bAlternative, bScript},
	} {
		vm, e := t.NewEngine(p2sh(entry.script), entry.tx, 0, t.ScriptVerifyCleanStack|t.ScriptVerifySigPushOnly|t.ScriptVerifyCheckSequenceVerify, 0, nil)
		if e == nil {
			e = vm.Execute()
		}
		if e != nil {
			tst.Fatalf("%s: %v", name, e)
		}
	}
	if bAlternative.TxIn[0].PreviousOutPoint != bLose.TxIn[0].PreviousOutPoint {
		tst.Fatal("not conflicting")
	}
	if bAlternative.TxHash() == bLose.TxHash() {
		tst.Fatal("not distinct branches")
	}
	tst.Log("Both B branches are individually valid; first-confirmed chain branch wins. Offchain acceptance alone does not revoke the losing branch's alternatives.")
}
