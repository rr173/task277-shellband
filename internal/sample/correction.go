package sample

import (
	"task277-shellband/internal/model"
)

// CorrectionParams 位置校正参数。
// Shrinkage 为切片制备收缩因子：corrected = raw × shrinkage（默认 1.0 即恒等变换）。
type CorrectionParams struct {
	Shrinkage float64
	Method    string
}

// DefaultCorrection 返回默认校正参数（恒等变换，方法标记为 linear-shrinkage）。
func DefaultCorrection() CorrectionParams {
	return CorrectionParams{Shrinkage: 1.0, Method: "linear-shrinkage"}
}

// CorrectPositions 对采样点做位置校正，返回校正记录列表。
// 关键不变量：按 sample_no 升序后，校正位置必须单调非降；
// 一旦出现倒序（corrected[i] < corrected[i-1]）即返回 ErrPositionOrder。
// 校正不改变采样点归属或状态，仅计算 corrected_pos 并产生校正记录。
func CorrectPositions(samples []*model.IsotopeSample, p CorrectionParams) ([]model.PositionCorrection, error) {
	if p.Shrinkage <= 0 {
		p.Shrinkage = 1.0
	}
	if p.Method == "" {
		p.Method = "linear-shrinkage"
	}
	ordered := SortByNumber(samples)
	corrs := make([]model.PositionCorrection, 0, len(ordered))
	var prev float64
	for i, sm := range ordered {
		cp := sm.RawPos * p.Shrinkage
		if i > 0 && cp < prev-1e-9 {
			return nil, model.ErrPositionOrder
		}
		corrs = append(corrs, model.PositionCorrection{
			BatchID:      sm.BatchID,
			SampleID:     sm.ID,
			RawPos:       sm.RawPos,
			CorrectedPos: cp,
			Method:       p.Method,
		})
		prev = cp
	}
	return corrs, nil
}

// ApplyCorrections 将校正位置写回采样点切片（就地修改 corrected_pos）。
// 调用方负责将结果持久化。corrs 与 samples 必须基于同一批次同一批采样点。
func ApplyCorrections(samples []*model.IsotopeSample, corrs []model.PositionCorrection) {
	byID := make(map[int64]float64, len(corrs))
	for _, c := range corrs {
		byID[c.SampleID] = c.CorrectedPos
	}
	for _, sm := range samples {
		if cp, ok := byID[sm.ID]; ok {
			sm.CorrectedPos = cp
		}
	}
}
