package wotsprobe

import "encoding/binary"

const Chains = 67

type Signature [Chains][32]byte
type PublicKey [Chains][32]byte

// Digits implements RFC8391 base_w(M,16,64) plus the three checksum digits.
func Digits(message [32]byte) [Chains]byte {
	var out [Chains]byte
	sum := 0
	for i, b := range message {
		out[2*i] = b >> 4
		out[2*i+1] = b & 15
		sum += 30 - int(out[2*i]) - int(out[2*i+1])
	}
	out[64] = byte(sum >> 8)
	out[65] = byte(sum>>4) & 15
	out[66] = byte(sum) & 15
	return out
}
func Chain(x, seed, address [32]byte, chain, start, steps int) [32]byte {
	if chain < 0 || chain >= Chains || start < 0 || steps < 0 || start+steps > 15 {
		panic("invalid RFC chain range")
	}
	binary.BigEndian.PutUint32(address[20:24], uint32(chain))
	for i := start; i < start+steps; i++ {
		binary.BigEndian.PutUint32(address[24:28], uint32(i))
		x = ReferenceStep(x, seed, address)
	}
	return x
}

// Sign and PublicFromSecret are research reference routines only. Secret OTS
// material MUST be securely generated and each secret key used for at most one
// message. This module does not implement durable one-time key management.
func Sign(message [32]byte, secret [Chains][32]byte, seed, address [32]byte) Signature {
	d := Digits(message)
	var sig Signature
	for i := range sig {
		sig[i] = Chain(secret[i], seed, address, i, 0, int(d[i]))
	}
	return sig
}
func PublicFromSecret(secret [Chains][32]byte, seed, address [32]byte) PublicKey {
	var pk PublicKey
	for i := range pk {
		pk[i] = Chain(secret[i], seed, address, i, 0, 15)
	}
	return pk
}
func Verify(message [32]byte, sig Signature, pk PublicKey, seed, address [32]byte) bool {
	d := Digits(message)
	for i := range sig {
		if Chain(sig[i], seed, address, i, int(d[i]), 15-int(d[i])) != pk[i] {
			return false
		}
	}
	return true
}
