package service

import (
	"path/filepath"
	"testing"

	"task277-shellband/internal/diagnose"
	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
	"task277-shellband/internal/store"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func TestHappyPathAlignDiagnoseSnapshot(t *testing.T) {
	svc := newSvc(t)
	b, err := svc.CreateBatch("T-HAPPY", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
		{BandIndex: 2, StartPos: 20, EndPos: 30},
	}); err != nil {
		t.Fatal(err)
	}
	created, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 12, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.9},
		{SampleNo: 3, RawPos: 22, IsotopeValue: -0.5, Unit: "per mil", RecrystallScore: 0.2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Align(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.AlignedCount() != 3 || res.GapCount() != 0 {
		t.Fatalf("align aligned=%d gap=%d", res.AlignedCount(), res.GapCount())
	}
	diag, err := svc.Diagnose(b.ID, diagnose.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(diag.Candidates) != 1 || diag.Candidates[0].SampleID != created[1].ID {
		t.Fatalf("diagnose %+v", diag.Candidates)
	}
	if err := svc.RecordVerdicts(b.ID, []*model.PollutionVerdict{
		{SampleID: created[1].ID, Verdict: "excluded", Reason: "artifact", Reviewer: "lab"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddAnchors(b.ID, []*model.AgeAnchor{
		{Position: 0, AgeYear: 2000, Source: "tl"},
		{Position: 30, AgeYear: 2003, Source: "tl"},
	}); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.BuildSnapshot(b.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Status != model.SnapshotPublished {
		t.Fatalf("status=%q", snap.Status)
	}
}

func TestSealWritesTimestamp(t *testing.T) {
	svc := newSvc(t)
	b, err := svc.CreateBatch("T-SEAL", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.TransitionStatus(b.ID, model.BatchPendingAlign); err != nil {
		t.Fatal(err)
	}
	if err := svc.TransitionStatus(b.ID, model.BatchNeedsReview); err != nil {
		t.Fatal(err)
	}
	if err := svc.TransitionStatus(b.ID, model.BatchPublished); err != nil {
		t.Fatal(err)
	}
	if err := svc.Seal(b.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.BatchSealed || got.SealedAt == nil {
		t.Fatalf("sealed batch=%+v", got)
	}
}

// seedForSnapshot 准备一个可对齐、可建快照的批次并返回其 id。
func seedForSnapshot(t *testing.T, svc *Service, code string) int64 {
	t.Helper()
	b, err := svc.CreateBatch(code, "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	return b.ID
}

// TestPublishDraftThenPublish 覆盖用户反馈的场景：
// 先保存草稿快照再发布。发布后草稿必须变成“已发布”，
// 而不能被直接标成“替代”，已发布列表中必须能查到该快照。
func TestPublishDraftThenPublish(t *testing.T) {
	svc := newSvc(t)
	bid := seedForSnapshot(t, svc, "T-PUB-DRAFT")

	draft, err := svc.BuildSnapshot(bid, false)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != model.SnapshotDraft {
		t.Fatalf("draft status=%q", draft.Status)
	}
	if err := svc.PublishSnapshot(draft.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got, err := svc.GetSnapshot(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.SnapshotPublished {
		t.Fatalf("after publish status=%q, want published (draft must become published, not superseded)", got.Status)
	}

	pub, err := svc.store.ListPublishedSnapshots(bid)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].ID != draft.ID {
		t.Fatalf("published list=%+v, want exactly the published draft", pub)
	}
}

// TestPublishSupersedesPreviousPublished 发布新草稿时应把旧已发布快照标为“替代”，
// 但不得把新发布的草稿也标成替代。
func TestPublishSupersedesPreviousPublished(t *testing.T) {
	svc := newSvc(t)
	bid := seedForSnapshot(t, svc, "T-PUB-SUP")

	old, err := svc.BuildSnapshot(bid, true)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := svc.BuildSnapshot(bid, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PublishSnapshot(draft.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	all, err := svc.ListSnapshots(bid)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]model.SnapshotStatus{}
	for _, s := range all {
		byID[s.ID] = s.Status
	}
	if byID[old.ID] != model.SnapshotSuperseded {
		t.Fatalf("old published=%q, want superseded", byID[old.ID])
	}
	if byID[draft.ID] != model.SnapshotPublished {
		t.Fatalf("new draft=%q, want published", byID[draft.ID])
	}

	pub, err := svc.store.ListPublishedSnapshots(bid)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].ID != draft.ID {
		t.Fatalf("published list=%+v, want only the newest published", pub)
	}
}

// TestPublishNonDraftRejected 非草稿快照不可发布，避免把“替代”态误发布。
func TestPublishNonDraftRejected(t *testing.T) {
	svc := newSvc(t)
	bid := seedForSnapshot(t, svc, "T-PUB-REJECT")

	old, err := svc.BuildSnapshot(bid, true)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := svc.BuildSnapshot(bid, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PublishSnapshot(draft.ID); err != nil {
		t.Fatal(err)
	}
	// old 现已为 superseded，再次发布必须报错而非谎报成功。
	if err := svc.PublishSnapshot(old.ID); err == nil {
		t.Fatal("expected error publishing superseded snapshot, got nil")
	}
}
