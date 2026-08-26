package slice

import (
	"testing"

	"task277-shellband/internal/model"
)

func TestBuildSectionSortsAndValidates(t *testing.T) {
	bands := []*model.GrowthBand{
		{ID: 2, BandIndex: 1, StartPos: 10, EndPos: 20},
		{ID: 1, BandIndex: 0, StartPos: 0, EndPos: 10},
	}
	sec := BuildSection(1, bands)
	if sec.Count() != 2 || sec.Bands[0].BandIndex != 0 {
		t.Fatalf("expected index order, got %+v", sec.Bands)
	}
	if err := sec.Validate(); err != nil {
		t.Fatal(err)
	}
	lo, hi := sec.Span()
	if lo != 0 || hi != 20 {
		t.Fatalf("span = %v,%v", lo, hi)
	}
}

func TestValidateRejectsOverlapAndEmpty(t *testing.T) {
	empty := BuildSection(1, nil)
	if err := empty.Validate(); err == nil {
		t.Fatal("empty section should fail")
	}
	overlap := BuildSection(1, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 9, EndPos: 20},
	})
	if err := overlap.Validate(); err == nil {
		t.Fatal("overlapping bands should fail")
	}
}

func TestLocateBandInsideAndOutside(t *testing.T) {
	sec := BuildSection(1, []*model.GrowthBand{
		{ID: 11, BandIndex: 0, StartPos: 0, EndPos: 10},
		{ID: 12, BandIndex: 1, StartPos: 10, EndPos: 20},
	})
	hit := sec.LocateBand(10)
	if hit == nil || hit.ID != 11 {
		t.Fatalf("endpoint should hit first band, got %+v", hit)
	}
	if sec.LocateBand(-1) != nil || sec.LocateBand(21) != nil {
		t.Fatal("outside section should be nil")
	}
}
