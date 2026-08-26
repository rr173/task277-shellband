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

func TestPublishedSnapshotIgnoresLaterAlign(t *testing.T) {
	svc := newTestService(t)
	b := mustShellBatch(t, svc, "SNAP-FREEZE")
	snap, err := svc.BuildSnapshot(b.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	frozen := payloadSampleCount(t, snap.Payload)
	if frozen == 0 {
		t.Fatal("published snapshot should contain aligned samples")
	}
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 4, RawPos: 38, IsotopeValue: 0.2, Unit: "per mil", RecrystallScore: 0.1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if payloadSampleCount(t, got.Payload) != frozen {
		t.Fatalf("published payload mutated: got %d want %d", payloadSampleCount(t, got.Payload), frozen)
	}
}
