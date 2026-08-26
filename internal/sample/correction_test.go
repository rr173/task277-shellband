package sample

import (
	"errors"
	"testing"

	"task277-shellband/internal/model"
)

func TestValidateAndDedupe(t *testing.T) {
	if err := Validate(&model.IsotopeSample{Unit: "", RecrystallScore: 0.2}); !errors.Is(err, model.ErrMissingUnit) {
		t.Fatalf("missing unit: %v", err)
	}
	ok := &model.IsotopeSample{SampleNo: 1, Unit: "per mil", RecrystallScore: 0.2}
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	in := []*model.IsotopeSample{
		{SampleNo: 1, Unit: "per mil"},
		{SampleNo: 1, Unit: "per mil"},
		{SampleNo: 2, Unit: "per mil"},
	}
	out, dup := DedupeByNumber(in)
	if len(out) != 2 || len(dup) != 1 || dup[0] != 1 {
		t.Fatalf("dedupe out=%d dup=%v", len(out), dup)
	}
}

func TestCorrectPositionsMonotonic(t *testing.T) {
	samples := []*model.IsotopeSample{
		{ID: 2, SampleNo: 2, RawPos: 4, BatchID: 1},
		{ID: 1, SampleNo: 1, RawPos: 1, BatchID: 1},
	}
	corrs, err := CorrectPositions(samples, DefaultCorrection())
	if err != nil {
		t.Fatal(err)
	}
	if len(corrs) != 2 || corrs[0].SampleID != 1 || corrs[0].CorrectedPos != 1 {
		t.Fatalf("expected number order, got %+v", corrs)
	}
	ApplyCorrections(samples, corrs)
	if samples[0].CorrectedPos != 4 || samples[1].CorrectedPos != 1 {
		t.Fatalf("apply should write by id, got %+v %+v", samples[0], samples[1])
	}
}

func TestCorrectPositionsRejectsInversion(t *testing.T) {
	samples := []*model.IsotopeSample{
		{ID: 1, SampleNo: 1, RawPos: 10, BatchID: 1},
		{ID: 2, SampleNo: 2, RawPos: 1, BatchID: 1},
	}
	_, err := CorrectPositions(samples, DefaultCorrection())
	if !errors.Is(err, model.ErrPositionOrder) {
		t.Fatalf("want ErrPositionOrder, got %v", err)
	}
}
