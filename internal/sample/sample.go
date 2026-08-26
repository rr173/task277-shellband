// Package sample 采样模块：负责氧同位素采样点的校验、幂等编号与位置校正。
package sample

import (
	"sort"

	"task277-shellband/internal/model"
)

// Validate 校验单条采样点：单位非空、重结晶评分落于 [0,1]。
func Validate(sm *model.IsotopeSample) error {
	if sm.Unit == "" {
		return model.ErrMissingUnit
	}
	if sm.RecrystallScore < 0 || sm.RecrystallScore > 1 {
		return model.NewDomainError("BAD_SCORE", "recrystall_score out of [0,1]", nil)
	}
	return nil
}

// DedupeByNumber 按 sample_no 去重，保留首次出现者，保证幂等编号。
// 返回去重后的采样点列表与被丢弃的重复编号。
func DedupeByNumber(samples []*model.IsotopeSample) ([]*model.IsotopeSample, []int64) {
	seen := map[int64]bool{}
	out := make([]*model.IsotopeSample, 0, len(samples))
	var dup []int64
	for _, sm := range samples {
		if seen[sm.SampleNo] {
			dup = append(dup, sm.SampleNo)
			continue
		}
		seen[sm.SampleNo] = true
		out = append(out, sm)
	}
	return out, dup
}

// SortByNumber 按采样编号升序排序（返回新切片，不改动入参）。
func SortByNumber(samples []*model.IsotopeSample) []*model.IsotopeSample {
	out := append([]*model.IsotopeSample{}, samples...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].SampleNo < out[j].SampleNo })
	return out
}
