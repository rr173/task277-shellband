// Package snapshot 快照模块：将批次的季节对齐结果固化为不可变快照载荷。
package snapshot

import (
	"encoding/json"
	"fmt"
	"sort"

	"task277-shellband/internal/model"
)

// excludedSet 由裁决构造“被排除采样点”集合（verdict=excluded 或 status=excluded）。
func excludedSet(samples []*model.IsotopeSample, verdicts []*model.PollutionVerdict) map[int64]bool {
	ex := map[int64]bool{}
	for _, sm := range samples {
		if sm.Status == model.SampleExcluded {
			ex[sm.ID] = true
		}
	}
	for _, v := range verdicts {
		if v.Verdict == "excluded" {
			ex[v.SampleID] = true
		}
	}
	return ex
}

// anchorAgeAt 依据年代锚点在给定位置线性插值出年代（无锚点或越界返回 nil）。
func anchorAgeAt(anchors []*model.AgeAnchor, pos float64) *float64 {
	if len(anchors) == 0 {
		return nil
	}
	sorted := append([]*model.AgeAnchor{}, anchors...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	if pos <= sorted[0].Position {
		a := sorted[0].AgeYear
		return &a
	}
	if pos >= sorted[len(sorted)-1].Position {
		a := sorted[len(sorted)-1].AgeYear
		return &a
	}
	for i := 0; i+1 < len(sorted); i++ {
		lo, hi := sorted[i], sorted[i+1]
		if pos >= lo.Position && pos <= hi.Position {
			if hi.Position == lo.Position {
				a := lo.AgeYear
				return &a
			}
			t := (pos - lo.Position) / (hi.Position - lo.Position)
			age := lo.AgeYear + t*(hi.AgeYear-lo.AgeYear)
			return &age
		}
	}
	return nil
}

// BuildPayload 构造季节快照载荷。
//
// 对每条带：统计归属采样点数量、在“非排除”采样点上的均值同位素，
// 并以年代锚点插值得到该带中点的年代（封存批次固定锚点）。被排除采样点单独列出。
func BuildPayload(batch *model.ShellBatch, bands []*model.GrowthBand, samples []*model.IsotopeSample,
	alignments []*model.Alignment, verdicts []*model.PollutionVerdict, anchors []*model.AgeAnchor) (*model.SnapshotPayload, error) {

	excluded := excludedSet(samples, verdicts)

	// 采样点按 id 索引，便于按归属聚合。
	sampleByID := map[int64]*model.IsotopeSample{}
	for _, sm := range samples {
		sampleByID[sm.ID] = sm
	}
	// 每条带 → 采样点列表。
	byBand := map[int64][]*model.IsotopeSample{}
	for _, a := range alignments {
		if sm, ok := sampleByID[a.SampleID]; ok {
			byBand[a.BandID] = append(byBand[a.BandID], sm)
		}
	}

	sortedBands := append([]*model.GrowthBand{}, bands...)
	sort.SliceStable(sortedBands, func(i, j int) bool {
		if sortedBands[i].BandIndex != sortedBands[j].BandIndex {
			return sortedBands[i].BandIndex < sortedBands[j].BandIndex
		}
		return sortedBands[i].StartPos < sortedBands[j].StartPos
	})

	payload := &model.SnapshotPayload{
		BatchCode:    batch.Code,
		GeneratedAt:  batch.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Bands:        make([]model.SnapshotBand, 0, len(sortedBands)),
		AnchorCount:  len(anchors),
		AnchorsSealed: batch.Status == model.BatchSealed,
	}
	for _, sm := range samples {
		if excluded[sm.ID] {
			payload.Excluded = append(payload.Excluded, sm.ID)
		}
	}

	for _, b := range sortedBands {
		sb := model.SnapshotBand{
			BandIndex: b.BandIndex,
			Kind:      b.Kind,
			StartPos:  b.StartPos,
			EndPos:    b.EndPos,
		}
		members := byBand[b.ID]
		sum := 0.0
		kept := 0
		for _, sm := range members {
			if excluded[sm.ID] {
				continue
			}
			sum += sm.IsotopeValue
			kept++
		}
		sb.SampleCount = len(members)
		if kept > 0 {
			sb.MeanIso = sum / float64(kept)
		}
		mid := (b.StartPos + b.EndPos) / 2.0
		if age := anchorAgeAt(anchors, mid); age != nil {
			a := *age
			sb.AgeYear = &a
		}
		payload.Bands = append(payload.Bands, sb)
	}
	return payload, nil
}

// Marshal 将载荷序列化为 JSON 字符串。
func Marshal(p *model.SnapshotPayload) (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", fmt.Errorf("snapshot: marshal payload: %w", err)
	}
	return string(b), nil
}
