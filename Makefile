.PHONY: check test desktop dev play preview replay reliability settings

GO ?= go
BIN ?= /tmp/stakewars-bin
SEED ?= 42

check:
	$(GO) test -race ./...
	$(GO) vet ./...
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -o $(BIN)/simvet ./cmd/simvet
	$(GO) vet -vettool=$(BIN)/simvet ./pkg/sim/...
	$(GO) build -buildvcs=false -tags desktop -o $(BIN)/stakewars ./cmd/stakewars
	$(GO) build -buildvcs=false -tags desktop,dev -o $(BIN)/stakewars-dev ./cmd/stakewars

test:
	$(GO) test ./...

desktop:
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -tags desktop -o $(BIN)/stakewars ./cmd/stakewars

dev:
	mkdir -p $(BIN)
	$(GO) build -buildvcs=false -tags desktop,dev -o $(BIN)/stakewars-dev ./cmd/stakewars

play: export GOCACHE ?= /tmp/stakewars-go-cache
play: dev
	$(BIN)/stakewars-dev -dev-arena -dev-seed $(SEED)

preview:
	mkdir -p artifacts
	$(GO) run ./cmd/stakewars-preview -output artifacts/stakewars-lobby.png
	$(GO) run ./cmd/stakewars-preview -arena -output artifacts/stakewars-arena.png

replay:
	$(GO) run ./cmd/stakewars-sim -verify pkg/replay/testdata/duel.json
	$(GO) run ./cmd/stakewars-sim -verify pkg/replay/testdata/six-squads.json
	$(GO) run ./cmd/stakewars-sim -verify pkg/replay/testdata/wide-six-squads.json

# Offline recovery, payment-script and disagreement counterexample tests.
reliability: export GOCACHE ?= /tmp/stakewars-go-cache
reliability:
	$(GO) test -race ./internal/durable ./internal/turnbatch ./internal/funding ./internal/protocolcheck ./internal/payout

# Open the lobby setup screen.
settings: export GOCACHE ?= /tmp/stakewars-go-cache
settings: dev
	$(BIN)/stakewars-dev -settings controls

# Interactive fictional lobby: inspect seat cards and cycle preparation states.
.PHONY: table-lobby
table-lobby: export GOCACHE ?= /tmp/stakewars-go-cache
table-lobby: dev
	$(BIN)/stakewars-dev -table-demo

# SDK seating recovery acceptance checks against a local mock bridge; no real money.
.PHONY: seating-check
seating-check: export GOCACHE ?= /tmp/stakewars-go-cache
seating-check:
	$(GO) test -race -v ./internal/seating

# Isolated covenant research; synthetic keys and UTXOs, no bridge or funds.
.PHONY: crypto-check crypto-policy-check
crypto-check: export GOCACHE ?= /tmp/stakewars-go-cache
crypto-check:
	$(GO) -C research/settlement test -mod=readonly -race -count=1 ./...
	$(GO) -C research/settlement vet -mod=readonly ./...
	python3 -m unittest discover -s research/settlement/claimmodel -p model.py -v

crypto-policy-check: export GOCACHE ?= /tmp/stakewars-go-cache
crypto-policy-check:
	GO="$(GO)" bash research/settlement/policy/check.sh

# Actual Poker escrow scripts and stock dcrd contextual refusal tests.
.PHONY: poker-exit-check
poker-exit-check: export GOCACHE ?= /tmp/stakewars-go-cache
poker-exit-check:
	GO="$(GO)" bash research/settlement/pokerexit/check.sh

# Real simulation trace narrowing; local research, no payment authorization.
.PHONY: turntrace-check turntrace-demo
turntrace-check: export GOCACHE ?= /tmp/stakewars-go-cache
turntrace-check:
	$(GO) test -race -count=1 -v ./research/turntrace/...
	$(GO) vet ./research/turntrace/...

turntrace-demo: export GOCACHE ?= /tmp/stakewars-go-cache
turntrace-demo:
	$(GO) run ./research/turntrace/cmd

# One actual flight arithmetic operation, compared with sim.Step.
.PHONY: flightproof-check
flightproof-check: export GOCACHE ?= /tmp/stakewars-go-cache
flightproof-check:
	$(GO) test -race -count=1 -v ./research/flightproof
	$(GO) vet ./research/flightproof

# Two complete desktop peers, local mTLS bridge, simulated funds only.
.PHONY: multiplayer-demo multiplayer-check
multiplayer-demo: export GOCACHE ?= /tmp/stakewars-go-cache
multiplayer-demo: dev
	$(GO) run -buildvcs=false ./cmd/stakewars-localbridge -desktop $(BIN)/stakewars-dev

multiplayer-check: export GOCACHE ?= /tmp/stakewars-go-cache
multiplayer-check:
	$(GO) test -race -count=1 -v ./internal/session ./internal/turnbatch ./internal/protocolcheck

# Two independent wallets, bridges and games on simnet: bond, seat draw, stake,
# a played match, a cooperative payout and a mature unilateral recovery. Builds
# the whole stack from the sibling source trees and needs docker. Nothing here
# touches mainnet. See docs/simnet.md.
.PHONY: acceptance
acceptance:
	bash simnet/run-financial-authority.sh
