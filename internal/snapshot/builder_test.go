package snapshot

import (
	"testing"

	"task277-shellband/internal/model"
)

func TestBuildPayloadExcludesVerdictsAndAgesBands(t *testing.T) {
	batch := &model.ShellBatch{Code: "S1", Status: model.BatchNeedsReview}
	bands := []*model.GrowthBand{
		{ID: 1, BandIndex: 0, StartPos: 0, EndPos: 10, Kind: model.BandContinuous},
		{ID: 2, BandIndex: 1, StartPos: 10, EndPos: 20, Kind: model.BandGap},
	}
	s1 := &model.IsotopeSample{ID: 11, IsotopeValue: 1.0, Status: model.SampleAligned}
	s2 := &model.IsotopeSample{ID: 12, IsotopeValue: 3.0, Status: model.SampleAligned}
	alignments := []*model.Alignment{
		{BandID: 1, SampleID: 11},
		{BandID: 1, SampleID: 12},
	}
	verdicts := []*model.PollutionVerdict{{SampleID: 12, Verdict: "excluded"}}
	anchors := []*model.AgeAnchor{
		{Position: 0, AgeYear: 2000},
		{Position: 20, AgeYear: 2010},
	}
	payload, err := BuildPayload(batch, bands, []*model.IsotopeSample{s1, s2}, alignments, verdicts, anchors)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Excluded) != 1 || payload.Excluded[0] != 12 {
		t.Fatalf("excluded=%v", payload.Excluded)
	}
	if payload.Bands[0].SampleCount != 2 {
		t.Fatalf("sample count=%d", payload.Bands[0].SampleCount)
	}
	if payload.Bands[0].MeanIso != 1.0 {
		t.Fatalf("mean should ignore excluded, got %v", payload.Bands[0].MeanIso)
	}
	if payload.Bands[1].Kind != model.BandGap {
		t.Fatalf("gap kind=%q", payload.Bands[1].Kind)
	}
	if payload.Bands[0].AgeYear == nil || *payload.Bands[0].AgeYear != 2002.5 {
		t.Fatalf("mid-band age=%v", payload.Bands[0].AgeYear)
	}
	if _, err := Marshal(payload); err != nil {
		t.Fatal(err)
	}
}
