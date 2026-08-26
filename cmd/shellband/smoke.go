package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"task277-shellband/internal/diagnose"
	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
	"task277-shellband/internal/service"
	"task277-shellband/internal/store"
)

// runSmokeTest 端到端自检：
// 1) 在临时文件库上跑完整流程（建批→带→采样→校正→对齐→诊断→裁决→锚点→快照）；
// 2) 关闭后重新打开同一文件库，验证计数与快照载荷一致（持久化 + 重启恢复）；
// 3) 全部断言通过返回 nil，否则返回首个失败原因。
func runSmokeTest() error {
	dir, err := os.MkdirTemp("", "shellband-smoke-")
	if err != nil {
		return fmt.Errorf("mkdtemp: %w", err)
	}
	defer os.RemoveAll(dir)
	dbFile := filepath.Join(dir, "smoke.db")

	st, err := store.Open(dbFile)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	svc := service.New(st)

	// 1) 建批 + 物种。
	batch, err := svc.CreateBatch("SMOKE-001", "Mercenaria mercenaria")
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	if batch.Status != model.BatchReceiving {
		return fmt.Errorf("unexpected initial status %q", batch.Status)
	}

	// 2) 四条生长带，band2 故意留空以制造缺口（缺失季节）。
	bands := []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
		{BandIndex: 2, StartPos: 20, EndPos: 30},
		{BandIndex: 3, StartPos: 30, EndPos: 40},
	}
	if _, err := svc.AddBands(batch.ID, bands); err != nil {
		return fmt.Errorf("add bands: %w", err)
	}

	// 3) 采样点：s2 同时具备重结晶与极端同位素（污染候选）。
	samples := []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 15, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.9},
		{SampleNo: 3, RawPos: 35, IsotopeValue: -1.0, Unit: "per mil", RecrystallScore: 0.2},
	}
	created, dup, err := svc.AddSamples(batch.ID, samples)
	if err != nil {
		return fmt.Errorf("add samples: %w", err)
	}
	if len(dup) != 0 {
		return fmt.Errorf("unexpected duplicates: %v", dup)
	}
	if len(created) != 3 {
		return fmt.Errorf("expected 3 samples, got %d", len(created))
	}
	var s2ID int64
	for _, sm := range created {
		if sm.SampleNo == 2 {
			s2ID = sm.ID
		}
	}

	// 4) 位置校正（默认恒等变换，要求单调非降）。
	if _, err := svc.Correct(batch.ID, sample.DefaultCorrection()); err != nil {
		return fmt.Errorf("correct: %w", err)
	}

	// 5) 对齐：band2 无采样 → 缺口。
	res, err := svc.Align(batch.ID)
	if err != nil {
		return fmt.Errorf("align: %w", err)
	}
	if res.GapCount() != 1 {
		return fmt.Errorf("expected 1 gap, got %d", res.GapCount())
	}
	if res.AlignedCount() != 3 {
		return fmt.Errorf("expected 3 aligned, got %d", res.AlignedCount())
	}

	// 6) 污染诊断：s2 应为候选。
	diag, err := svc.Diagnose(batch.ID, diagnose.DefaultOptions())
	if err != nil {
		return fmt.Errorf("diagnose: %w", err)
	}
	if len(diag.Candidates) != 1 || diag.Candidates[0].SampleID != s2ID {
		return fmt.Errorf("expected 1 pollution candidate (sample %d), got %+v", s2ID, diag.Candidates)
	}

	// 7) 裁决：排除 s2。
	if err := svc.RecordVerdicts(batch.ID, []*model.PollutionVerdict{
		{SampleID: s2ID, Verdict: "excluded", Reason: "recrystallization artifact", Reviewer: "smoke"},
	}); err != nil {
		return fmt.Errorf("record verdicts: %w", err)
	}

	// 8) 年代锚点（用于快照定年）。
	if err := svc.AddAnchors(batch.ID, []*model.AgeAnchor{
		{Position: 0, AgeYear: 2020, Source: "tl"},
		{Position: 40, AgeYear: 2021, Source: "tl"},
	}); err != nil {
		return fmt.Errorf("add anchors: %w", err)
	}

	// 9) 发布季节快照。
	snap, err := svc.BuildSnapshot(batch.ID, true)
	if err != nil {
		return fmt.Errorf("build snapshot: %w", err)
	}
	if snap.Status != model.SnapshotPublished {
		return fmt.Errorf("expected published snapshot, got %q", snap.Status)
	}
	var payload model.SnapshotPayload
	if err := json.Unmarshal([]byte(snap.Payload), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	if len(payload.Bands) != 4 {
		return fmt.Errorf("expected 4 bands in payload, got %d", len(payload.Bands))
	}
	if len(payload.Excluded) != 1 || payload.Excluded[0] != s2ID {
		return fmt.Errorf("expected excluded sample %d, got %v", s2ID, payload.Excluded)
	}
	// 校验 band2 为缺口。
	if payload.Bands[2].Kind != model.BandGap {
		return fmt.Errorf("expected band2 kind=gap, got %q", payload.Bands[2].Kind)
	}

	// 10) 关闭后重开，验证持久化与重启恢复。
	if err := st.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	st2, err := store.Open(dbFile)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer st2.Close()
	svc2 := service.New(st2)

	stats, err := svc2.Stats()
	if err != nil {
		return fmt.Errorf("stats after reopen: %w", err)
	}
	if stats.Batches != 1 || stats.Bands != 4 || stats.Samples != 3 ||
		stats.Anchors != 2 || stats.Alignments != 3 || stats.Verdicts != 1 ||
		stats.Snapshots != 1 || stats.PublishedSnaps != 1 {
		return fmt.Errorf("persistence mismatch after reopen: %+v", stats)
	}
	reSnaps, err := svc2.ListSnapshots(batch.ID)
	if err != nil {
		return fmt.Errorf("list snapshots after reopen: %w", err)
	}
	if len(reSnaps) != 1 || reSnaps[0].Status != model.SnapshotPublished {
		return fmt.Errorf("snapshot not persisted correctly: %+v", reSnaps)
	}
	return nil
}
