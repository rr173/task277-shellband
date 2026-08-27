package service

import (
	"encoding/json"
	"fmt"
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

// TestConcurrentAlignPublishNoZeroCount 验证“对齐与快照发布互不踩踏”不变量：
// 二十个并发协程反复对同一切片做对齐，与此同时另一批协程反复发布季节快照。
// 任何一次发布成功的快照里，归属采样计数都不得为零（对齐清空归属的窗口
// 不得被快照读走）。回归模拟同事并发的踩踏场景。
func TestConcurrentAlignPublishNoZeroCount(t *testing.T) {
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
	// 三个采样点分别归属三条带，对齐后每条带恰好 1 个采样点。
	if _, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 12, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 3, RawPos: 22, IsotopeValue: -0.5, Unit: "per mil", RecrystallScore: 0.1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	// 先做一次对齐，确保有归属可供快照统计。
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}

	const aligners = 10
	const publishers = 10
	const iterations = 50

	var wg sync.WaitGroup
	var alignErr, pubErr error
	var errMu sync.Mutex
	recordErr := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if err != nil && alignErr == nil {
			alignErr = err
		}
	}
	recordPubErr := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if err != nil && pubErr == nil {
			pubErr = err
		}
	}

	// 对齐协程：反复重对齐。
	wg.Add(aligners)
	for i := 0; i < aligners; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if _, err := svc.Align(b.ID); err != nil {
					recordErr(err)
					return
				}
			}
		}()
	}

	// 发布协程：反复构造并发布快照，并断言采样计数非零。
	wg.Add(publishers)
	for i := 0; i < publishers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				snap, err := svc.BuildSnapshot(b.ID, true)
				if err != nil {
					// 并发下偶发的版本冲突可接受，但其它错误须上抛。
					if de, ok := err.(*model.DomainError); ok && de.Code == "DUP_VERSION" {
						continue
					}
					recordPubErr(err)
					return
				}
				var payload model.SnapshotPayload
				if err := json.Unmarshal([]byte(snap.Payload), &payload); err != nil {
					recordPubErr(err)
					return
				}
				// 三条带各 1 个采样点 → 总采样计数应为 3，绝不为 0。
				total := 0
				for _, band := range payload.Bands {
					total += band.SampleCount
				}
				if total == 0 {
					recordPubErr(fmt.Errorf("snapshot %d has zero sample count after concurrent align", snap.ID))
					return
				}
			}
		}()
	}
	wg.Wait()
	if alignErr != nil {
		t.Fatalf("align error: %v", alignErr)
	}
	if pubErr != nil {
		t.Fatalf("publish error: %v", pubErr)
	}

	// 收尾：表内归属总数应与采样点一致（每采样点归属一条带）。
	st, err := svc.BatchStats(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Alignments != 3 {
		t.Fatalf("expected 3 alignments after concurrent align, got %d", st.Alignments)
	}
}
