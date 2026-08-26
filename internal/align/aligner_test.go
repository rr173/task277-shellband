package align

import (
	"testing"

	"task277-shellband/internal/model"
	"task277-shellband/internal/slice"
)

func TestAlignMarksGapAndAssignments(t *testing.T) {
	sec := slice.BuildSection(1, []*model.GrowthBand{
		{ID: 1, BatchID: 1, BandIndex: 0, StartPos: 0, EndPos: 10},
		{ID: 2, BatchID: 1, BandIndex: 1, StartPos: 10, EndPos: 20},
		{ID: 3, BatchID: 1, BandIndex: 2, StartPos: 20, EndPos: 30},
	})
	samples := []*model.IsotopeSample{
		{ID: 11, BatchID: 1, SampleNo: 1, CorrectedPos: 2},
		{ID: 12, BatchID: 1, SampleNo: 2, CorrectedPos: 25},
		{ID: 13, BatchID: 1, SampleNo: 3, CorrectedPos: 99},
	}
	res := Align(sec, samples)
	if res.GapCount() != 1 {
		t.Fatalf("gap=%d", res.GapCount())
	}
	if res.AlignedCount() != 2 {
		t.Fatalf("aligned=%d", res.AlignedCount())
	}
	if res.BandKind[2] != model.BandGap {
		t.Fatalf("middle band should be gap, got %q", res.BandKind[2])
	}
	if res.SampleStatus[13] != model.SampleMissing {
		t.Fatalf("out-of-section sample status=%q", res.SampleStatus[13])
	}
	if len(res.OutOfSection) != 1 || res.OutOfSection[0] != 13 {
		t.Fatalf("out of section=%v", res.OutOfSection)
	}
}
