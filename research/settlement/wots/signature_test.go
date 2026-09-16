package wotsprobe

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	t "github.com/decred/dcrd/txscript/v4"
	"testing"
)

func TestArbitraryDigestSignatureAndScriptSteps(tst *testing.T) {
	var sk [Chains][32]byte
	for i := range sk {
		sk[i] = sha256.Sum256([]byte(fmt.Sprintf("fixture-not-production-key-%d", i)))
	}
	seed := sha256.Sum256([]byte("public-seed"))
	var address [32]byte
	binary.BigEndian.PutUint32(address[16:20], 7)
	message := sha256.Sum256([]byte("arbitrary checkpoint digest"))
	sig := Sign(message, sk, seed, address)
	pk := PublicFromSecret(sk, seed, address)
	if !Verify(message, sig, pk, seed, address) {
		tst.Fatal("reference signature failed")
	}
	digits := Digits(message)
	stepCount := 0
	for i := range sig {
		x := sig[i]
		for step := int(digits[i]); step < 15; step++ {
			var adrs = address
			binary.BigEndian.PutUint32(adrs[20:24], uint32(i))
			binary.BigEndian.PutUint32(adrs[24:28], uint32(step))
			expected := ReferenceStep(x, seed, adrs)
			b := t.NewScriptBuilder()
			EmitStep(b, seed, adrs)
			b.AddData(expected[:]).AddOp(t.OP_EQUAL)
			script, e := b.Script()
			if e != nil {
				tst.Fatal(e)
			}
			if e = execute(script, x); e != nil {
				tst.Fatalf("chain%d step%d: %v", i, step, e)
			}
			x = expected
			stepCount++
		}
		if x != pk[i] {
			tst.Fatal("public key mismatch")
		}
	}
	tst.Logf("all67 chains including checksum verified; %d actual RFC chain steps", stepCount)
	wrong := message
	wrong[0] ^= 1
	if Verify(wrong, sig, pk, seed, address) {
		tst.Fatal("wrong message accepted")
	}
	changed := sig
	changed[66][0] ^= 1
	if Verify(message, changed, pk, seed, address) {
		tst.Fatal("bad checksum-chain signature accepted")
	}
	wrongSeed := seed
	wrongSeed[0] ^= 1
	if Verify(message, sig, pk, wrongSeed, address) {
		tst.Fatal("wrong public seed accepted")
	}
	wrongAddress := address
	wrongAddress[19] ^= 1
	if Verify(message, sig, pk, seed, wrongAddress) {
		tst.Fatal("wrong OTS address accepted")
	}
}

func TestChecksumExtremes(tst *testing.T) {
	var z [32]byte
	d := Digits(z)
	if d[64] != 3 || d[65] != 12 || d[66] != 0 {
		tst.Fatal(d[64:])
	}
	steps := 0
	for _, v := range d {
		steps += 15 - int(v)
	}
	if steps != 990 {
		tst.Fatalf("zero-message step count %d", steps)
	}
	var f [32]byte
	for i := range f {
		f[i] = 255
	}
	d = Digits(f)
	if d[64] != 0 || d[65] != 0 || d[66] != 0 {
		tst.Fatal(d[64:])
	}
	steps = 0
	for _, v := range d {
		steps += 15 - int(v)
	}
	if steps != 45 {
		tst.Fatalf("all-ones message step count %d", steps)
	}
}
