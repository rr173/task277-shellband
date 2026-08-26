// Package align 对齐模块：将采样点归属到生长带，并诊断缺失季节（缺口）。
package align

import (
	"sort"

	"task277-shellband/internal/model"
	"task277-shellband/internal/slice"
)

// Result 对齐结果：采样点归属、各带判定种类与备注、落在切片外的采样点。
type Result struct {
	Assignments  []model.Alignment
	BandKind     map[int64]model.BandKind
	BandNote     map[int64]string
	SampleStatus map[int64]model.SampleStatus
	OutOfSection []int64
}

// Align 将已校正位置的采样点对齐到切片生长带。
//
// 规则：
//   - 采样点校正位置落在某条带区间内 → 归属该带、状态 aligned；
//   - 落在所有带区间外（切片首尾之外） → 状态 missing，不入任何带；
//   - 一条带若含 ≥1 采样点 → 连续（continuous），否则 → 缺口（gap，缺失季节）。
//
// 同一切片对齐串行执行（由 service 层加锁保证），本函数本身为纯计算。
func Align(sec *slice.Section, samples []*model.IsotopeSample) *Result {
	res := &Result{
		BandKind:     map[int64]model.BandKind{},
		BandNote:     map[int64]string{},
		SampleStatus: map[int64]model.SampleStatus{},
	}
	sorted := append([]*model.IsotopeSample{}, samples...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].SampleNo < sorted[j].SampleNo })

	// 统计每条带的采样点数量。
	count := map[int64]int{}
	for _, sm := range sorted {
		band := sec.LocateBand(sm.CorrectedPos)
		if band == nil {
			res.SampleStatus[sm.ID] = model.SampleMissing
			res.OutOfSection = append(res.OutOfSection, sm.ID)
			continue
		}
		res.Assignments = append(res.Assignments, model.Alignment{
			BatchID:  sm.BatchID,
			BandID:   band.ID,
			SampleID: sm.ID,
		})
		res.SampleStatus[sm.ID] = model.SampleAligned
		count[band.ID]++
	}

	// 判定每条带种类：有采样 → continuous，无采样 → gap。
	for _, b := range sec.Bands {
		if count[b.ID] > 0 {
			res.BandKind[b.ID] = model.BandContinuous
			res.BandNote[b.ID] = "aligned with samples"
		} else {
			res.BandKind[b.ID] = model.BandGap
			res.BandNote[b.ID] = "missing season (no samples)"
		}
	}
	return res
}

// GapCount 返回缺口（缺失季节）数量。
func (r *Result) GapCount() int {
	n := 0
	for _, k := range r.BandKind {
		if k == model.BandGap {
			n++
		}
	}
	return n
}

// AlignedCount 返回成功归属到带的采样点数量。
func (r *Result) AlignedCount() int {
	return len(r.Assignments)
}
