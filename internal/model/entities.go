// Package model 定义贝壳同位素季节生长带对齐服务的领域实体、状态枚举与错误。
//
// 业务域：古气候研究者将贝壳切片的氧同位素采样序列对齐到生长带，
// 以辨别缺失季节（缺口）与切片重结晶造成的假信号（污染）。
// 本包不含持久化与 I/O，仅承载领域结构与不变量校验。
package model

import "time"

// BatchStatus 贝壳批次状态机：
// 接收中 → 待对齐 → 需复核 → 已发布 → 封存。
type BatchStatus string

const (
	BatchReceiving    BatchStatus = "receiving"
	BatchPendingAlign BatchStatus = "pending_align"
	BatchNeedsReview  BatchStatus = "needs_review"
	BatchPublished    BatchStatus = "published"
	BatchSealed       BatchStatus = "sealed"
)

// ValidBatchStatus 返回合法批次状态集合。
func ValidBatchStatus() map[BatchStatus]bool {
	return map[BatchStatus]bool{
		BatchReceiving:    true,
		BatchPendingAlign: true,
		BatchNeedsReview:  true,
		BatchPublished:    true,
		BatchSealed:       true,
	}
}

// CanTransition 判定批次状态是否允许从 from 流转到 to。
// 封存为终态，封存后任何流转均禁止（封存批次只读）。
func (s BatchStatus) CanTransition(to BatchStatus) bool {
	if s == BatchSealed {
		return false
	}
	order := []BatchStatus{BatchReceiving, BatchPendingAlign, BatchNeedsReview, BatchPublished, BatchSealed}
	fromIdx, toIdx := -1, -1
	for i, v := range order {
		if v == s {
			fromIdx = i
		}
		if v == to {
			toIdx = i
		}
	}
	if fromIdx < 0 || toIdx < 0 {
		return false
	}
	// 只允许向前推进，不允许回退（已发布不可退回需复核）。
	return toIdx > fromIdx
}

// SampleStatus 采样点状态：原始 / 已对齐 / 重结晶 / 缺失 / 排除。
type SampleStatus string

const (
	SampleRaw            SampleStatus = "raw"
	SampleAligned        SampleStatus = "aligned"
	SampleRecrystallized SampleStatus = "recrystallized"
	SampleMissing        SampleStatus = "missing"
	SampleExcluded       SampleStatus = "excluded"
)

// BandKind 季节带判定种类：候选 / 连续 / 缺口 / 确认 / 否决。
type BandKind string

const (
	BandCandidate BandKind = "candidate"
	BandContinuous BandKind = "continuous"
	BandGap       BandKind = "gap"
	BandConfirmed BandKind = "confirmed"
	BandRejected  BandKind = "rejected"
)

// SnapshotStatus 季节快照状态：草稿 / 发布 / 替代。
type SnapshotStatus string

const (
	SnapshotDraft      SnapshotStatus = "draft"
	SnapshotPublished  SnapshotStatus = "published"
	SnapshotSuperseded SnapshotStatus = "superseded"
)

// ShellBatch 贝壳批次，一次切片分析的基本单位。
type ShellBatch struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Species   string     `json:"species"`
	Status    BatchStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	SealedAt  *time.Time `json:"sealed_at,omitempty"`
}

// GrowthBand 单条季节生长带边界，沿切片位置轴定义一段区间。
type GrowthBand struct {
	ID        int64     `json:"id"`
	BatchID   int64     `json:"batch_id"`
	BandIndex int       `json:"band_index"`
	StartPos  float64   `json:"start_pos"`
	EndPos    float64   `json:"end_pos"`
	Kind      BandKind  `json:"kind"`
	Note      string    `json:"note,omitempty"`
}

// Contains 判定给定校正后位置是否落在生长带区间内（含端点）。
func (b GrowthBand) Contains(pos float64) bool {
	return pos >= b.StartPos-1e-9 && pos <= b.EndPos+1e-9
}

// IsPeriodic 判定两条相邻生长带是否构成周期对齐（区间连续且长度相等）。
func BandsPeriodic(a, b GrowthBand) bool {
	if a.BandIndex < 0 || b.BandIndex < 0 {
		return false
	}
	if b.BandIndex != a.BandIndex+1 {
		return false
	}
	if a.EndPos < a.StartPos || b.EndPos < b.StartPos {
		return false
	}
	la, lb := a.EndPos-a.StartPos, b.EndPos-b.StartPos
	if la <= 0 || lb <= 0 {
		return false
	}
	ratio := lb / la
	return ratio > 0.5 && ratio < 2.0
}

// IsotopeSample 氧同位素采样点，沿切片位置轴采集。
type IsotopeSample struct {
	ID              int64        `json:"id"`
	BatchID         int64        `json:"batch_id"`
	SampleNo        int64        `json:"sample_no"`
	RawPos          float64      `json:"raw_pos"`
	CorrectedPos    float64      `json:"corrected_pos"`
	IsotopeValue    float64      `json:"isotope_value"`
	Unit            string       `json:"unit"`
	RecrystallScore float64      `json:"recrystall_score"`
	Status          SampleStatus `json:"status"`
	BandID          *int64       `json:"band_id,omitempty"`
}

// AgeAnchor 年代锚点，将切片位置映射到日历年代，用于快照定年。
type AgeAnchor struct {
	ID       int64   `json:"id"`
	BatchID  int64   `json:"batch_id"`
	Position float64 `json:"position"`
	AgeYear  float64 `json:"age_year"`
	Source   string  `json:"source"`
}

// PositionCorrection 采样位置校正记录，保留校正前后位置与方法。
type PositionCorrection struct {
	ID           int64     `json:"id"`
	BatchID      int64     `json:"batch_id"`
	SampleID     int64     `json:"sample_id"`
	RawPos       float64   `json:"raw_pos"`
	CorrectedPos float64   `json:"corrected_pos"`
	Method       string    `json:"method"`
	AppliedAt    time.Time `json:"applied_at"`
}

// Alignment 采样点与生长带的归属关系（一个采样点归属一条带）。
type Alignment struct {
	ID      int64 `json:"id"`
	BatchID int64 `json:"batch_id"`
	BandID  int64 `json:"band_id"`
	SampleID int64 `json:"sample_id"`
}

// PollutionVerdict 重结晶污染裁决，研究者对污染候选的人工判定。
type PollutionVerdict struct {
	ID       int64     `json:"id"`
	BatchID  int64     `json:"batch_id"`
	SampleID int64     `json:"sample_id"`
	Verdict  string    `json:"verdict"` // excluded | kept
	Reason   string    `json:"reason"`
	Reviewer string    `json:"reviewer"`
	At       time.Time `json:"at"`
}

// SeasonalSnapshot 季节快照，发布后不可变（除非被替代），封存时固定年代锚点。
type SeasonalSnapshot struct {
	ID        int64           `json:"id"`
	BatchID   int64           `json:"batch_id"`
	Version   int             `json:"version"`
	Status    SnapshotStatus  `json:"status"`
	Sealed    bool            `json:"sealed"`
	Payload   string          `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// SnapshotBand 快照内单条季节带的摘要视图。
type SnapshotBand struct {
	BandIndex  int         `json:"band_index"`
	Kind       BandKind    `json:"kind"`
	StartPos   float64     `json:"start_pos"`
	EndPos     float64     `json:"end_pos"`
	SampleCount int        `json:"sample_count"`
	MeanIso    float64     `json:"mean_iso"`
	AgeYear    *float64    `json:"age_year,omitempty"`
}

// SnapshotPayload 季节快照的载荷结构，序列化进 SeasonalSnapshot.Payload。
type SnapshotPayload struct {
	BatchCode   string         `json:"batch_code"`
	GeneratedAt string         `json:"generated_at"`
	Bands       []SnapshotBand `json:"bands"`
	Excluded    []int64        `json:"excluded_samples"`
	AnchorCount int            `json:"anchor_count"`
	AnchorsSealed bool         `json:"anchors_sealed"`
}
