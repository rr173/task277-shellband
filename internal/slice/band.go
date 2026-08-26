package slice

import "task277-shellband/internal/model"

// LocateBand 返回包含给定校正后位置的生长带（按索引顺序首条命中）。
// 若位置落在所有带区间外（切片首尾之外）返回 nil。
func (s *Section) LocateBand(pos float64) *model.GrowthBand {
	for _, b := range s.Bands {
		if b.Contains(pos) {
			return b
		}
	}
	return nil
}

// PeriodicSegments 返回相邻且周期一致的生长带对数量，用于连续性判定。
// 两条带索引连续、区间合法且长度相等（ratio 落在 0.5~2.0）视为周期一致。
func (s *Section) PeriodicSegments() int {
	n := 0
	for i := 0; i+1 < len(s.Bands); i++ {
		if model.BandsPeriodic(*s.Bands[i], *s.Bands[i+1]) {
			n++
		}
	}
	return n
}

// ContinuityRatio 已定义带中“与后一条周期一致”的比例，用于闭环不变量陈述。
func (s *Section) ContinuityRatio() float64 {
	if len(s.Bands) <= 1 {
		return 1.0
	}
	return float64(s.PeriodicSegments()) / float64(len(s.Bands)-1)
}
