// Package slice 切片模块：负责贝壳切片的结构组织与生长带边界管理。
// 生长带是沿切片位置轴排布的周期区间，承载季节信号的连续性与缺口判定。
package slice

import (
	"sort"

	"task277-shellband/internal/model"
)

// Section 切片视图，聚合生长带边界并校验其结构一致性。
type Section struct {
	BatchID int64
	Bands   []*model.GrowthBand
}

// BuildSection 由批次的生长带列表构造切片视图，并按索引升序排序。
func BuildSection(batchID int64, bands []*model.GrowthBand) *Section {
	sorted := make([]*model.GrowthBand, len(bands))
	copy(sorted, bands)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].BandIndex != sorted[j].BandIndex {
			return sorted[i].BandIndex < sorted[j].BandIndex
		}
		return sorted[i].StartPos < sorted[j].StartPos
	})
	return &Section{BatchID: batchID, Bands: sorted}
}

// Validate 校验切片结构：至少一条带、区间合法、索引从 0 连续、互不重叠。
func (s *Section) Validate() error {
	if len(s.Bands) == 0 {
		return model.ErrNoBands
	}
	seen := map[int]bool{}
	for i, b := range s.Bands {
		if b.BandIndex < 0 {
			return model.NewDomainError("BAD_INDEX", "negative band index", nil)
		}
		if seen[b.BandIndex] {
			return model.NewDomainError("DUP_INDEX", "duplicate band index", nil)
		}
		seen[b.BandIndex] = true
		if b.EndPos <= b.StartPos {
			return model.NewDomainError("BAD_RANGE", "band end_pos must exceed start_pos", nil)
		}
		if i > 0 && b.StartPos < s.Bands[i-1].EndPos-1e-9 {
			return model.NewDomainError("OVERLAP", "growth bands overlap", nil)
		}
	}
	// 索引须从 0 连续递增。
	for i := 0; i < len(s.Bands); i++ {
		if !seen[i] {
			return model.NewDomainError("GAP_INDEX", "band index not contiguous from 0", nil)
		}
	}
	return nil
}

// Count 返回生长带数量。
func (s *Section) Count() int { return len(s.Bands) }

// Span 返回切片覆盖的位置范围 [min, max]。
func (s *Section) Span() (float64, float64) {
	if len(s.Bands) == 0 {
		return 0, 0
	}
	min, max := s.Bands[0].StartPos, s.Bands[0].EndPos
	for _, b := range s.Bands {
		if b.StartPos < min {
			min = b.StartPos
		}
		if b.EndPos > max {
			max = b.EndPos
		}
	}
	return min, max
}
