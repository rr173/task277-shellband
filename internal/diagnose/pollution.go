// Package diagnose 诊断模块：基于显微重结晶证据，标记氧同位素异常值的污染候选。
package diagnose

import (
	"sort"

	"task277-shellband/internal/model"
)

// Options 污染诊断阈值。
// RecrystallThreshold：重结晶评分达到该值视为重结晶区（默认 0.6）。
// IsoExtreme：|同位素值| 达到该值视为极端（疑似真实季节极端或污染，默认 2.5）。
type Options struct {
	RecrystallThreshold float64
	IsoExtreme          float64
}

// DefaultOptions 返回默认诊断阈值。
func DefaultOptions() Options {
	return Options{RecrystallThreshold: 0.6, IsoExtreme: 2.5}
}

// Candidate 单条污染候选（重结晶导致假信号）。
type Candidate struct {
	SampleID         int64   `json:"sample_id"`
	RecrystallScore  float64 `json:"recrystall_score"`
	IsotopeValue     float64 `json:"isotope_value"`
	Reason           string  `json:"reason"`
}

// Result 诊断结果。
type Result struct {
	Candidates []Candidate `json:"candidates"`
}

// Diagnose 对采样点做重结晶污染诊断。
//
// 判定：recrystall_score ≥ 阈值 且 |isotope_value| ≥ 极端阈值 → 污染候选。
// 理由：异常高/低同位素值若同时伴随重结晶显微证据，更可能是切片重结晶造成的
// 假信号，而非真实季节极端（真实极端应无明显重结晶证据）。仅标记候选，
// 是否排除由研究者裁决（verdict）决定。仅对已对齐采样点诊断。
func Diagnose(samples []*model.IsotopeSample, opts Options) *Result {
	if opts.RecrystallThreshold <= 0 {
		opts = DefaultOptions()
	}
	if opts.IsoExtreme <= 0 {
		opts = DefaultOptions()
	}
	out := &Result{}
	ordered := append([]*model.IsotopeSample{}, samples...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SampleNo < ordered[j].SampleNo })
	for _, sm := range ordered {
		if sm.Status != model.SampleAligned {
			continue
		}
		if sm.RecrystallScore >= opts.RecrystallThreshold && abs(sm.IsotopeValue) >= opts.IsoExtreme {
			out.Candidates = append(out.Candidates, Candidate{
				SampleID:        sm.ID,
				RecrystallScore: sm.RecrystallScore,
				IsotopeValue:    sm.IsotopeValue,
				Reason:          "recrystallization evidence co-occurs with isotope extreme",
			})
		}
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
