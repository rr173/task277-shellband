package service

import (
	"encoding/json"
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

// TestPublishedSnapshotImmutableAfterRealign 回归：
// 发布季节快照后又补采样、重新对齐，再读取已发布快照，
// 载荷必须保持发布当时的序列，不得跟随最新对齐变动。
func TestPublishedSnapshotImmutableAfterRealign(t *testing.T) {
	svc := newSvc(t)
	b, err := svc.CreateBatch("T-IMMUT", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
	}); err != nil {
		t.Fatal(err)
	}
	// 初始：仅 band0 有 1 个采样点，band1 为缺口。
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.BuildSnapshot(b.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	publishedPayload := snap.Payload
	// 发布当时的序列：4 条带中 band1 应为缺口。
	var pre model.SnapshotPayload
	if err := json.Unmarshal([]byte(publishedPayload), &pre); err != nil {
		t.Fatal(err)
	}
	if pre.Bands[1].Kind != model.BandGap || pre.Bands[1].SampleCount != 0 {
		t.Fatalf("expected band1 gap at publish time, got %+v", pre.Bands[1])
	}
	if pre.Bands[0].SampleCount != 1 || pre.Bands[0].MeanIso != 1.0 {
		t.Fatalf("expected band0 to hold 1 sample mean=1.0 at publish time, got %+v", pre.Bands[0])
	}

	// 补采样进 band1 并重新对齐：band1 不再是缺口。
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 2, RawPos: 15, IsotopeValue: 5.0, Unit: "per mil", RecrystallScore: 0.2},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}

	// 再读取已发布快照：载荷必须与发布当时完全一致。
	got, err := svc.GetSnapshot(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Payload != publishedPayload {
		t.Fatalf("published snapshot payload mutated after re-align\nwant: %s\ngot:  %s",
			publishedPayload, got.Payload)
	}
	// 列表读取路径同样不得改写已发布快照。
	listed, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Status != model.SnapshotPublished {
		t.Fatalf("expected 1 published snapshot, got %+v", listed)
	}
	if listed[0].Payload != publishedPayload {
		t.Fatalf("listed published payload mutated after re-align\nwant: %s\ngot:  %s",
			publishedPayload, listed[0].Payload)
	}
}

// TestDraftSnapshotRefreshesFromLive 草稿快照是可刷新的工作预览：
// 读回时应反映最新活体数据。
func TestDraftSnapshotRefreshesFromLive(t *testing.T) {
	svc := newSvc(t)
	b, err := svc.CreateBatch("T-DRAFT", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.BuildSnapshot(b.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != model.SnapshotDraft {
		t.Fatalf("expected draft, got %q", draft.Status)
	}
	// 补采样进 band1 并重新对齐：草稿读回应反映 band1 由缺口变连续。
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 2, RawPos: 15, IsotopeValue: 5.0, Unit: "per mil", RecrystallScore: 0.2},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetSnapshot(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	var p model.SnapshotPayload
	if err := json.Unmarshal([]byte(got.Payload), &p); err != nil {
		t.Fatal(err)
	}
	if p.Bands[1].Kind != model.BandContinuous || p.Bands[1].SampleCount != 1 {
		t.Fatalf("draft should reflect latest alignment, got %+v", p.Bands[1])
	}
}
