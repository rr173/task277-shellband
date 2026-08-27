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

func TestListSnapshotsSeesNewVersions(t *testing.T) {
	svc := newTestService(t)
	b := mustShellBatch(t, svc, "SNAP-CACHE")
	if _, err := svc.BuildSnapshot(b.ID, true); err != nil {
		t.Fatal(err)
	}
	first, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("after first publish want 1, got %d", len(first))
	}
	if _, err := svc.BuildSnapshot(b.ID, true); err != nil {
		t.Fatal(err)
	}
	second, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("after second publish want 2 snapshots, got %d", len(second))
	}
}
