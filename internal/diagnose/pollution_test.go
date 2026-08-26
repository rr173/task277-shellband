package diagnose

import (
	"testing"

	"task277-shellband/internal/model"
)

func TestDiagnoseRequiresAlignedAndBothThresholds(t *testing.T) {
	samples := []*model.IsotopeSample{
		{ID: 1, SampleNo: 1, Status: model.SampleAligned, RecrystallScore: 0.9, IsotopeValue: 3.0},
		{ID: 2, SampleNo: 2, Status: model.SampleAligned, RecrystallScore: 0.1, IsotopeValue: 3.0},
		{ID: 3, SampleNo: 3, Status: model.SampleRaw, RecrystallScore: 0.9, IsotopeValue: 3.0},
		{ID: 4, SampleNo: 4, Status: model.SampleAligned, RecrystallScore: 0.9, IsotopeValue: 0.2},
	}
	res := Diagnose(samples, DefaultOptions())
	if len(res.Candidates) != 1 || res.Candidates[0].SampleID != 1 {
		t.Fatalf("candidates=%+v", res.Candidates)
	}
}
