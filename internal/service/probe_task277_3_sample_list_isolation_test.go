package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
	"task277-shellband/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func mustShellBatch(t *testing.T, svc *Service, code string) *model.ShellBatch {
	t.Helper()
	b, err := svc.CreateBatch(code, "Pecten maximus")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
		{BandIndex: 2, StartPos: 20, EndPos: 30},
		{BandIndex: 3, StartPos: 30, EndPos: 40},
	}); err != nil {
		t.Fatal(err)
	}
	created, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 15, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.9},
		{SampleNo: 3, RawPos: 35, IsotopeValue: -1.0, Unit: "per mil", RecrystallScore: 0.2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	_ = created
	return b
}

func payloadSampleCount(t *testing.T, raw string) int {
	t.Helper()
	var payload model.SnapshotPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range payload.Bands {
		n += b.SampleCount
	}
	return n
}

func TestSampleListsDoNotShareBackingArray(t *testing.T) {
	svc := newTestService(t)
	a := mustShellBatch(t, svc, "SLICE-A")
	b := mustShellBatch(t, svc, "SLICE-B")
	first, err := svc.ListSamples(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 3 {
		t.Fatalf("want 3 samples in A, got %d", len(first))
	}
	aid := first[0].BatchID
	second, err := svc.ListSamples(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 {
		t.Fatal("batch B should have samples")
	}
	if first[0].BatchID != aid || first[0].BatchID != a.ID {
		t.Fatalf("list A was overwritten: first batch=%d want=%d; B batch=%d", first[0].BatchID, a.ID, second[0].BatchID)
	}
	for _, sm := range first {
		if sm.BatchID != a.ID {
			t.Fatalf("list A contains foreign sample %+v", sm)
		}
	}
}
