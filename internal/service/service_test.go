package service

import (
	"path/filepath"
	"sync"
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

// TestConcurrentAlignPreservesInvariant 模拟二十位同事同时对同一批次对齐：
// 全部调用必须成功（无 duplicate-alignment / 半写错误），且收尾后
// “一条样本仅归属一条带”不变量成立——归属数恰等于采样点数。
func TestConcurrentAlignPreservesInvariant(t *testing.T) {
	svc := newSvc(t)
	b, err := svc.CreateBatch("T-CONC", "Pecten")
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
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 12, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.9},
		{SampleNo: 3, RawPos: 22, IsotopeValue: -0.5, Unit: "per mil", RecrystallScore: 0.2},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}

	const workers = 20
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			if _, e := svc.Align(b.ID); e != nil {
				errs[i] = e
			}
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("worker %d align failed: %v", i, e)
		}
	}

	as, err := svc.ListAlignments(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 不变量：每个采样点最终恰好归属一条带。
	seen := map[int64]int64{}
	for _, a := range as {
		if prev, dup := seen[a.SampleID]; dup {
			t.Fatalf("sample %d assigned to both band %d and band %d", a.SampleID, prev, a.BandID)
		}
		seen[a.SampleID] = a.BandID
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 unique sample assignments, got %d", len(seen))
	}
}
