module github.com/karamble/dcrgaming-stakewars

go 1.25.0

require (
	github.com/decred/dcrd/chaincfg/chainhash v1.0.5
	github.com/decred/dcrd/chaincfg/v3 v3.3.0
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0
	github.com/decred/dcrd/txscript/v4 v4.1.2
	github.com/decred/dcrd/wire v1.7.2
	github.com/hajimehoshi/ebiten/v2 v2.9.9
	github.com/karamble/dcrgaming-sdk v0.0.0
	golang.org/x/image v0.31.0
	golang.org/x/tools v0.44.0
	lukechampine.com/blake3 v1.3.0
)

require (
	decred.org/dcrwallet/v5 v5.0.2 // indirect
	github.com/decred/dcrd/blockchain/standalone/v2 v2.2.2 // indirect
	github.com/decred/dcrd/crypto/rand v1.0.1 // indirect
	golang.org/x/crypto v0.51.0 // indirect
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/decred/base58 v1.0.6 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.1.0
	github.com/decred/dcrd/crypto/ripemd160 v1.0.2
	github.com/decred/dcrd/dcrec v1.0.1 // indirect
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.4 // indirect
	github.com/decred/dcrd/dcrutil/v4 v4.0.3
	github.com/decred/slog v1.2.0
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/jessevdk/go-flags v1.6.1
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/jrick/logrotate v1.1.2
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

// Development harness uses the sibling checkout. No production release pin yet.
replace github.com/karamble/dcrgaming-sdk => ../dcrgaming-sdk
