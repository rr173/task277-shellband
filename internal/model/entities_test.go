package model

import "testing"

func TestCanTransitionForwardOnly(t *testing.T) {
	if !BatchReceiving.CanTransition(BatchPendingAlign) {
		t.Fatal("receiving should advance to pending_align")
	}
	if BatchPendingAlign.CanTransition(BatchReceiving) {
		t.Fatal("status must not move backward")
	}
	if BatchSealed.CanTransition(BatchPublished) {
		t.Fatal("sealed is terminal")
	}
	if !BatchPublished.CanTransition(BatchSealed) {
		t.Fatal("published should advance to sealed")
	}
}

func TestBandContainsEndpoints(t *testing.T) {
	b := GrowthBand{StartPos: 10, EndPos: 20}
	if !b.Contains(10) || !b.Contains(20) || !b.Contains(15) {
		t.Fatal("band should include endpoints")
	}
	if b.Contains(9.9) || b.Contains(20.1) {
		t.Fatal("band should reject positions outside the interval")
	}
}

func TestBandsPeriodicAdjacent(t *testing.T) {
	a := GrowthBand{BandIndex: 0, StartPos: 0, EndPos: 10}
	b := GrowthBand{BandIndex: 1, StartPos: 10, EndPos: 20}
	if !BandsPeriodic(a, b) {
		t.Fatal("adjacent equal-length bands should be periodic")
	}
	c := GrowthBand{BandIndex: 3, StartPos: 20, EndPos: 30}
	if BandsPeriodic(b, c) {
		t.Fatal("non-consecutive indexes should not be periodic")
	}
}
