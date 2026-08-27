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

func TestPublishDraftBecomesPublished(t *testing.T) {
	svc := newTestService(t)
	b := mustShellBatch(t, svc, "SNAP-PUB")
	draft, err := svc.BuildSnapshot(b.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != model.SnapshotDraft {
		t.Fatalf("status=%q", draft.Status)
	}
	if err := svc.PublishSnapshot(draft.ID); err != nil {
		t.Fatal(err)
	}
	all, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	published := 0
	for _, snap := range all {
		if snap.Status == model.SnapshotPublished {
			published++
		}
		if snap.ID == draft.ID && snap.Status != model.SnapshotPublished {
			t.Fatalf("draft %d became %q", snap.ID, snap.Status)
		}
	}
	if published != 1 {
		t.Fatalf("want 1 published snapshot, got %d from %+v", published, all)
	}
}
