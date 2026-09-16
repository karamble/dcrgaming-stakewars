package main

import "testing"

// Root funding has no direct exit. Every funder must possess and durably retain
// the complete genesis endorsement BEFORE releasing its funding signature.
func TestWalletFundingRequiresCompleteGenesisCertificate(t *testing.T) {
	tx, r := checkpointPrepare(1, 2)
	checkpointAuthorize(tx, r, r, 2, secrets[0][0][:], nil)
	if e := checkpointVerify(tx, r); e == nil {
		t.Fatal("partial genesis certificate unexpectedly authorizes claim")
	}
	checkpointAuthorize(tx, r, r, 2, secrets[0][0][:], secrets[1][0][:])
	if e := checkpointVerify(tx, r); e != nil {
		t.Fatal(e)
	}
}
