package seating

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	"github.com/karamble/dcrgaming-stakewars/internal/payout"
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"lukechampine.com/blake3"
)

// AnchorConfirmations is part of the signed preset. The seating beacon is
// already an agreed future block, so it can seed the seat draw and battlefield
// as soon as that block exists. A changed anchor never replaces a previously
// signed world; it requires a new table.
const AnchorConfirmations uint32 = 1
const ContentVersion = "stakewars-builtins-v17"

type Manifest struct {
	Version             uint32
	Simulation          uint32
	Content             string
	Config              sim.Config
	Payout              payout.Policy
	AnchorConfirmations uint32
	Settlement          string
}

func Preset(seats uint32, stake uint64) (Manifest, error) {
	p, err := payout.WinnerTakesAll(uint8(seats), int64(stake))
	if err != nil {
		return Manifest{}, err
	}
	c := sim.DefaultConfig()
	c.Seed = 0
	c.Seats = uint8(seats)
	m := Manifest{Version: 2, Simulation: sim.Version, Content: ContentVersion, Config: c, Payout: p, AnchorConfirmations: AnchorConfirmations, Settlement: "bridge-fee-cooperative-payout-owner-refund-v2"}
	return m, m.Config.Validate()
}
func (m Manifest) Hash() ([32]byte, error) {
	if err := m.Payout.Validate(); err != nil {
		return [32]byte{}, err
	}
	if err := m.Config.Validate(); err != nil {
		return [32]byte{}, err
	}
	if m.Config.Seed != 0 || m.Config.Seats != m.Payout.Seats {
		return [32]byte{}, fmt.Errorf("invalid manifest seed or seats")
	}
	// Struct fields and allocation rows have fixed ordering; no maps or floats.
	raw, err := json.Marshal(m)
	if err != nil {
		return [32]byte{}, err
	}
	return blake3.Sum256(append([]byte("StakeWars/manifest/v2\x00"), raw...)), nil
}

type World struct {
	Match                       string
	Terms, Rules, Anchor, State [32]byte
	AnchorHeight                uint32
	Seed                        uint64
}

func BuildWorld(terms membership.Terms, roster, anchor string) (World, *sim.State, error) {
	var w World
	rosterRaw, err := hex.DecodeString(roster)
	if err != nil || len(rosterRaw) != 32 {
		return w, nil, fmt.Errorf("invalid roster hash")
	}
	anchorRaw, err := hex.DecodeString(anchor)
	if err != nil || len(anchorRaw) != 32 {
		return w, nil, fmt.Errorf("invalid anchor hash")
	}
	manifest, err := Preset(terms.Seats, terms.BuyInAtoms)
	if err != nil {
		return w, nil, err
	}
	hash, err := manifest.Hash()
	if err != nil {
		return w, nil, err
	}
	if terms.GameVer != ProtocolVersion || terms.Game != "stakewars" {
		return w, nil, fmt.Errorf("game rules differ from signed table terms")
	}
	termsHash, err := terms.Hash()
	if err != nil {
		return w, nil, err
	}
	seedInput := append([]byte("StakeWars/map/v1\x00"), anchorRaw...)
	seedInput = append(seedInput, rosterRaw...)
	seedInput = append(seedInput, hash[:]...)
	digest := blake3.Sum256(seedInput)
	seed := binary.LittleEndian.Uint64(digest[:8])
	config := manifest.Config
	config.Seed = seed
	state, err := sim.New(config)
	if err != nil {
		return w, nil, err
	}
	w = World{Match: roster, Terms: termsHash, Rules: hash, AnchorHeight: membership.BeaconHeight(terms), Seed: seed, State: replay.Hash(state)}
	copy(w.Anchor[:], anchorRaw)
	return w, state, nil
}
func (w World) Hash() [32]byte {
	raw, _ := json.Marshal(w)
	return blake3.Sum256(append([]byte("StakeWars/world/v1\x00"), raw...))
}
