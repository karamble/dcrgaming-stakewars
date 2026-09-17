package seating

import "testing"

func TestSeatingBeaconNeedsNoExtraConfirmationBlocks(t *testing.T) {
	if AnchorConfirmations != 1 {
		t.Fatalf("anchor confirmations = %d, want 1", AnchorConfirmations)
	}
}
