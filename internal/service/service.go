// Package service 编排层：将 store 持久化与 slice/sample/align/diagnose/snapshot
// 业务算法组合成可复用的领域操作。所有写操作在单写连接上串行；
// 针对“对齐/快照”等需要全局串行的不变量，额外加 service 级互斥锁。
package service

import (
	"fmt"
	"sync"
	"time"

	"task277-shellband/internal/align"
	"task277-shellband/internal/diagnose"
	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
	"task277-shellband/internal/slice"
	"task277-shellband/internal/snapshot"
	"task277-shellband/internal/store"
)

// Service 封装 store 与业务包，对外提供高层的批次分析操作。
type Service struct {
	store *store.Store
	// serialMu 保证同进程内“对齐/快照”全局串行，避免并发写入导致的不一致。
	serialMu sync.Mutex
}

// New 构造 Service。
func New(s *store.Store) *Service {
	return &Service{store: s}
}

// ---- 批次 ----

// CreateBatch 创建贝壳批次（初始 receiving 状态）。
func (svc *Service) CreateBatch(code, species string) (*model.ShellBatch, error) {
	if code == "" {
		return nil, model.NewDomainError("BAD_CODE", "batch code is empty", nil)
	}
	return svc.store.CreateBatch(code, species)
}

// GetBatch 读取批次。
func (svc *Service) GetBatch(id int64) (*model.ShellBatch, error) {
	return svc.store.GetBatch(id)
}

// ListBatches 列出全部批次。
func (svc *Service) ListBatches() ([]*model.ShellBatch, error) {
	return svc.store.ListBatches()
}

// SetSpecies 设置物种信息（任何非封存状态均允许）。
func (svc *Service) SetSpecies(id int64, species string) error {
	b, err := svc.store.GetBatch(id)
	if err != nil {
		return err
	}
	if b.Status == model.BatchSealed {
		return model.NewDomainError("SEALED", "sealed batch is read-only", nil)
	}
	return svc.store.SetBatchSpecies(id, species)
}

// TransitionStatus 按状态机推进批次状态（仅允许向前、封存为终态）。
func (svc *Service) TransitionStatus(id int64, to model.BatchStatus) error {
	if !model.ValidBatchStatus()[to] {
		return model.NewDomainError("BAD_STATUS", fmt.Sprintf("unknown status %q", to), nil)
	}
	b, err := svc.store.GetBatch(id)
	if err != nil {
		return err
	}
	if !b.Status.CanTransition(to) {
		return model.NewDomainError("ILLEGAL_TRANSITION",
			fmt.Sprintf("cannot transition %s -> %s", b.Status, to), nil)
	}
	return svc.store.UpdateBatchStatus(id, to, nil)
}

// Seal 封存批次：状态机推进到 sealed 并写入封存时间。封存后只读。
func (svc *Service) Seal(id int64) error {
	b, err := svc.store.GetBatch(id)
	if err != nil {
		return err
	}
	if b.Status == model.BatchSealed {
		return nil
	}
	if !b.Status.CanTransition(model.BatchSealed) {
		return model.NewDomainError("ILLEGAL_TRANSITION",
			fmt.Sprintf("cannot seal from %s", b.Status), nil)
	}
	now := time.Now().UTC()
	return svc.store.UpdateBatchStatus(id, model.BatchSealed, &now)
}

// ---- 生长带 ----

// AddBands 批量新增生长带（初始候选 kind）。调用方应保证索引从 0 连续递增。
func (svc *Service) AddBands(batchID int64, bands []*model.GrowthBand) ([]*model.GrowthBand, error) {
	out := make([]*model.GrowthBand, 0, len(bands))
	for _, b := range bands {
		b.BatchID = batchID
		created, err := svc.store.CreateBand(b)
		if err != nil {
			return nil, err
		}
		out = append(out, created)
	}
	return out, nil
}

// ListBands 列出批次全部生长带。
func (svc *Service) ListBands(batchID int64) ([]*model.GrowthBand, error) {
	return svc.store.ListBands(batchID)
}

// ---- 采样 ----

// AddSamples 批量新增采样点：先校验单位与重结晶评分，按编号去重保证幂等。
// 返回落库采样点与被丢弃的重复编号。
func (svc *Service) AddSamples(batchID int64, samples []*model.IsotopeSample) ([]*model.IsotopeSample, []int64, error) {
	deduped, dup := sample.DedupeByNumber(samples)
	out := make([]*model.IsotopeSample, 0, len(deduped))
	for _, sm := range deduped {
		if err := sample.Validate(sm); err != nil {
			return nil, nil, err
		}
		sm.BatchID = batchID
		created, err := svc.store.CreateSample(sm)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, created)
	}
	return out, dup, nil
}

// ListSamples 列出批次全部采样点。
func (svc *Service) ListSamples(batchID int64) ([]*model.IsotopeSample, error) {
	return svc.store.ListSamples(batchID)
}

// ---- 年代锚点 ----

// AddAnchors 批量新增年代锚点（同批次内 position 唯一）。
func (svc *Service) AddAnchors(batchID int64, anchors []*model.AgeAnchor) error {
	for _, a := range anchors {
		a.BatchID = batchID
		if _, err := svc.store.CreateAnchor(a); err != nil {
			return err
		}
	}
	return nil
}

// ListAnchors 列出批次全部锚点。
func (svc *Service) ListAnchors(batchID int64) ([]*model.AgeAnchor, error) {
	return svc.store.ListAnchors(batchID)
}

// ---- 位置校正 ----

// Correct 对批次采样点做位置校正：计算校正位置、持久化校正记录、回写 corrected_pos。
func (svc *Service) Correct(batchID int64, p sample.CorrectionParams) ([]model.PositionCorrection, error) {
	samples, err := svc.store.ListSamples(batchID)
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, model.NewDomainError("NO_SAMPLES", "no samples to correct", nil)
	}
	corrs, err := sample.CorrectPositions(samples, p)
	if err != nil {
		return nil, err
	}
	for _, c := range corrs {
		rec := c
		if err := svc.store.CreateCorrection(&rec); err != nil {
			return nil, err
		}
	}
	sample.ApplyCorrections(samples, corrs)
	for _, sm := range samples {
		if err := svc.store.SetCorrectedPos(sm.ID, sm.CorrectedPos); err != nil {
			return nil, err
		}
	}
	return corrs, nil
}

// ListCorrections 列出批次全部校正记录。
func (svc *Service) ListCorrections(batchID int64) ([]*model.PositionCorrection, error) {
	return svc.store.ListCorrections(batchID)
}

// ---- 对齐（全局串行）----

// Align 将采样点对齐到生长带并诊断缺口；写回归属、带种类与采样状态。
// 该操作全局串行，避免并发对齐破坏“一条样本仅归属一条带”的不变量。
func (svc *Service) Align(batchID int64) (*align.Result, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()

	bands, err := svc.store.ListBands(batchID)
	if err != nil {
		return nil, err
	}
	sec := slice.BuildSection(batchID, bands)
	if err := sec.Validate(); err != nil {
		return nil, err
	}
	samples, err := svc.store.ListSamples(batchID)
	if err != nil {
		return nil, err
	}
	res := align.Align(sec, samples)

	// 清空旧归属，重对齐为幂等操作。
	if err := svc.store.DeleteAlignments(batchID); err != nil {
		return nil, err
	}
	for _, a := range res.Assignments {
		asg := a
		if err := svc.store.CreateAlignment(&asg); err != nil {
			return nil, err
		}
	}
	for _, b := range sec.Bands {
		kind := res.BandKind[b.ID]
		note := res.BandNote[b.ID]
		if err := svc.store.UpdateBandKind(b.ID, kind, note); err != nil {
			return nil, err
		}
	}
	for _, sm := range samples {
		st, ok := res.SampleStatus[sm.ID]
		if !ok {
			continue
		}
		if err := svc.store.UpdateSampleStatus(sm.ID, st, nil); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// ListAlignments 列出批次全部归属关系。
func (svc *Service) ListAlignments(batchID int64) ([]*model.Alignment, error) {
	return svc.store.ListAlignments(batchID)
}

// ---- 污染诊断 ----

// Diagnose 对批次采样点做重结晶污染诊断，返回污染候选。
func (svc *Service) Diagnose(batchID int64, opts diagnose.Options) (*diagnose.Result, error) {
	samples, err := svc.store.ListSamples(batchID)
	if err != nil {
		return nil, err
	}
	return diagnose.Diagnose(samples, opts), nil
}

// ---- 污染裁决 ----

// RecordVerdicts 批量记录对某批次采样点的污染裁决（覆盖式）。
func (svc *Service) RecordVerdicts(batchID int64, verdicts []*model.PollutionVerdict) error {
	for _, v := range verdicts {
		v.BatchID = batchID
		if v.Verdict != "excluded" && v.Verdict != "kept" {
			return model.NewDomainError("BAD_VERDICT", "verdict must be excluded|kept", nil)
		}
		if err := svc.store.UpsertVerdict(v); err != nil {
			return err
		}
	}
	return nil
}

// ListVerdicts 列出批次全部污染裁决。
func (svc *Service) ListVerdicts(batchID int64) ([]*model.PollutionVerdict, error) {
	return svc.store.ListVerdicts(batchID)
}

// ---- 季节快照 ----

// BuildSnapshot 构造季节快照载荷并落库。publish=true 时先替代已发布快照再发布。
// 该操作全局串行，与 Align 互斥，避免快照读取到半写状态。
func (svc *Service) BuildSnapshot(batchID int64, publish bool) (*model.SeasonalSnapshot, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()

	b, err := svc.store.GetBatch(batchID)
	if err != nil {
		return nil, err
	}
	bands, err := svc.store.ListBands(batchID)
	if err != nil {
		return nil, err
	}
	samples, err := svc.store.ListSamples(batchID)
	if err != nil {
		return nil, err
	}
	alignments, err := svc.store.ListAlignments(batchID)
	if err != nil {
		return nil, err
	}
	verdicts, err := svc.store.ListVerdicts(batchID)
	if err != nil {
		return nil, err
	}
	anchors, err := svc.store.ListAnchors(batchID)
	if err != nil {
		return nil, err
	}
	payload, err := snapshot.BuildPayload(b, bands, samples, alignments, verdicts, anchors)
	if err != nil {
		return nil, err
	}
	marshaled, err := snapshot.Marshal(payload)
	if err != nil {
		return nil, err
	}
	version, err := svc.store.NextSnapshotVersion(batchID)
	if err != nil {
		return nil, err
	}
	status := model.SnapshotDraft
	if publish {
		status = model.SnapshotPublished
		if err := svc.store.SupersedePublished(batchID); err != nil {
			return nil, err
		}
	}
	snap := &model.SeasonalSnapshot{
		BatchID: batchID,
		Version: version,
		Status:  status,
		Sealed:  b.Status == model.BatchSealed,
		Payload: marshaled,
	}
	return svc.store.CreateSnapshot(snap)
}

// ListSnapshots 列出批次全部快照。
func (svc *Service) refreshSnapshotFromLive(snap *model.SeasonalSnapshot) error {
	b, err := svc.store.GetBatch(snap.BatchID)
	if err != nil {
		return err
	}
	bands, err := svc.store.ListBands(snap.BatchID)
	if err != nil {
		return err
	}
	samples, err := svc.store.ListSamples(snap.BatchID)
	if err != nil {
		return err
	}
	alignments, err := svc.store.ListAlignments(snap.BatchID)
	if err != nil {
		return err
	}
	verdicts, err := svc.store.ListVerdicts(snap.BatchID)
	if err != nil {
		return err
	}
	anchors, err := svc.store.ListAnchors(snap.BatchID)
	if err != nil {
		return err
	}
	payload, err := snapshot.BuildPayload(b, bands, samples, alignments, verdicts, anchors)
	if err != nil {
		return err
	}
	marshaled, err := snapshot.Marshal(payload)
	if err != nil {
		return err
	}
	snap.Payload = marshaled
	return nil
}

func (svc *Service) ListSnapshots(batchID int64) ([]*model.SeasonalSnapshot, error) {
	snaps, err := svc.store.ListSnapshots(batchID)
	if err != nil {
		return nil, err
	}
	for _, snap := range snaps {
		if err := svc.refreshSnapshotFromLive(snap); err != nil {
			return nil, err
		}
	}
	return snaps, nil
}

// GetSnapshot 读取单个快照。
func (svc *Service) GetSnapshot(id int64) (*model.SeasonalSnapshot, error) {
	snap, err := svc.store.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if err := svc.refreshSnapshotFromLive(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// PublishSnapshot 将已有草稿快照发布（先替代已发布者）。
func (svc *Service) PublishSnapshot(id int64) error {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()
	snap, err := svc.store.GetSnapshot(id)
	if err != nil {
		return err
	}
	if snap.Status == model.SnapshotPublished {
		return nil
	}
	if err := svc.store.SupersedePublished(snap.BatchID); err != nil {
		return err
	}
	// 直接更新状态为 published（保持版本号不变）。
	res, err := svc.store.DB.Exec(
		`UPDATE seasonal_snapshots SET status=? WHERE id=? AND status=?`,
		string(model.SnapshotPublished), id, string(model.SnapshotDraft))
	if err != nil {
		return fmt.Errorf("service: publish snapshot: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrSnapshotNotFound
	}
	return nil
}

// ---- 统计 ----

// Stats 返回全库计数。
func (svc *Service) Stats() (*store.Stats, error) {
	return svc.store.GlobalStats()
}

// BatchStats 返回单批次计数。
func (svc *Service) BatchStats(batchID int64) (*store.Stats, error) {
	return svc.store.BatchStats(batchID)
}
