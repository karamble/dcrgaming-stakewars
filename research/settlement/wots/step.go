package wotsprobe

import (
	"crypto/sha256"
	t "github.com/decred/dcrd/txscript/v4"
)

// EmitStep consumes one32-byte X and produces the RFC8391 SHA2-256 chain step.
// SEED and ADRS are parameters committed by the containing script. The caller
// must enforce the correct signer, chain and step addresses, and verify all
// message/checksum chains. This fragment is not a complete signature verifier.
func EmitStep(b *t.ScriptBuilder, seed, address [32]byte) {
	prefix := make([]byte, 32)
	prefix[31] = 3
	prefix = append(prefix, seed[:]...)
	address[28] = 0
	address[29] = 0
	address[30] = 0
	prefix = append(prefix, address[:31]...)
	b.AddData(prefix)
	emitPreparedStep(b)
}

// EmitDynamicStep consumes X32, SEED32, ADRS32 and produces Y32. Caller must bind
// SEED/ADRS to the signer key, immutable message and exact verifier position.
// Per RFC8391 this function replaces the whole keyAndMask word with0 then1.
func EmitDynamicStep(b *t.ScriptBuilder) {
	b.AddOp(t.OP_SIZE).AddInt64(32).AddOp(t.OP_EQUALVERIFY)
	b.AddOp(t.OP_OVER).AddOp(t.OP_SIZE).AddInt64(32).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_DROP)
	b.AddInt64(2).AddOp(t.OP_PICK).AddOp(t.OP_SIZE).AddInt64(32).AddOp(t.OP_EQUALVERIFY).AddOp(t.OP_DROP)
	b.AddInt64(28).AddOp(t.OP_LEFT).AddData([]byte{0, 0, 0}).AddOp(t.OP_CAT).AddOp(t.OP_CAT)
	prefix := make([]byte, 32)
	prefix[31] = 3
	b.AddData(prefix).AddOp(t.OP_SWAP).AddOp(t.OP_CAT)
	emitPreparedStep(b)
}

func emitPreparedStep(b *t.ScriptBuilder) {
	b.AddOp(t.OP_DUP).AddData([]byte{0, 0}).AddInt64(1).AddOp(t.OP_LEFT).AddOp(t.OP_CAT).AddOp(t.OP_SHA256).AddOp(t.OP_TOALTSTACK)
	b.AddData([]byte{1}).AddOp(t.OP_CAT).AddOp(t.OP_SHA256).AddOp(t.OP_0)
	// stack X,mask,accumulator. Three-byte unsigned chunks are padded with a
	// high01 sentinel, yielding canonical positive4-byte ScriptNums. XOR cancels
	// that sentinel. Padding the result then slicing recovers fixed-width bytes.
	for start := 0; start < 32; start += 3 {
		end := start + 3
		if end > 32 {
			end = 32
		}
		width := end - start
		b.AddInt64(2).AddOp(t.OP_PICK).AddInt64(int64(end)).AddInt64(int64(start)).AddOp(t.OP_SUBSTR).AddData([]byte{1}).AddOp(t.OP_CAT)
		b.AddInt64(2).AddOp(t.OP_PICK).AddInt64(int64(end)).AddInt64(int64(start)).AddOp(t.OP_SUBSTR).AddData([]byte{1}).AddOp(t.OP_CAT)
		b.AddOp(t.OP_XOR).AddData([]byte{0, 0, 0}).AddOp(t.OP_CAT).AddInt64(int64(width)).AddOp(t.OP_LEFT).AddOp(t.OP_CAT)
	}
	b.AddOp(t.OP_TOALTSTACK).AddOp(t.OP_2DROP).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_FROMALTSTACK).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddData(make([]byte, 32)).AddOp(t.OP_SWAP).AddOp(t.OP_CAT).AddOp(t.OP_SHA256)
}

func ReferenceStep(x, seed, address [32]byte) [32]byte {
	var p [96]byte
	p[31] = 3
	copy(p[32:64], seed[:])
	address[28] = 0
	address[29] = 0
	address[30] = 0
	address[31] = 0
	copy(p[64:], address[:])
	key := sha256.Sum256(p[:])
	p[95] = 1
	mask := sha256.Sum256(p[:])
	p = [96]byte{}
	copy(p[32:64], key[:])
	for i := 0; i < 32; i++ {
		p[64+i] = x[i] ^ mask[i]
	}
	return sha256.Sum256(p[:])
}
