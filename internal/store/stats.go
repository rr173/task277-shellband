package store

import (
	"fmt"
)

// Stats 汇总批次层面的计数指标，供 API /stats 与端到端断言使用。
type Stats struct {
	Batches        int `json:"batches"`
	Bands          int `json:"bands"`
	Samples        int `json:"samples"`
	Anchors        int `json:"anchors"`
	Alignments     int `json:"alignments"`
	Verdicts       int `json:"verdicts"`
	Snapshots      int `json:"snapshots"`
	PublishedSnaps int `json:"published_snapshots"`
}

// GlobalStats 返回全库计数。
func (s *Store) GlobalStats() (*Stats, error) {
	var st Stats
	queries := []struct {
		dst *int
		q   string
	}{
		{&st.Batches, `SELECT COUNT(*) FROM shell_batches`},
		{&st.Bands, `SELECT COUNT(*) FROM growth_bands`},
		{&st.Samples, `SELECT COUNT(*) FROM isotope_samples`},
		{&st.Anchors, `SELECT COUNT(*) FROM age_anchors`},
		{&st.Alignments, `SELECT COUNT(*) FROM alignments`},
		{&st.Verdicts, `SELECT COUNT(*) FROM pollution_verdicts`},
		{&st.Snapshots, `SELECT COUNT(*) FROM seasonal_snapshots`},
		{&st.PublishedSnaps, `SELECT COUNT(*) FROM seasonal_snapshots WHERE status='published'`},
	}
	for _, item := range queries {
		if err := s.DB.QueryRow(item.q).Scan(item.dst); err != nil {
			return nil, fmt.Errorf("store: global stats: %w", err)
		}
	}
	return &st, nil
}

// BatchStats 返回单个批次的计数指标。
func (s *Store) BatchStats(batchID int64) (*Stats, error) {
	var st Stats
	st.Batches = 1
	queries := []struct {
		dst *int
		q   string
	}{
		{&st.Bands, `SELECT COUNT(*) FROM growth_bands WHERE batch_id=?`},
		{&st.Samples, `SELECT COUNT(*) FROM isotope_samples WHERE batch_id=?`},
		{&st.Anchors, `SELECT COUNT(*) FROM age_anchors WHERE batch_id=?`},
		{&st.Alignments, `SELECT COUNT(*) FROM alignments WHERE batch_id=?`},
		{&st.Verdicts, `SELECT COUNT(*) FROM pollution_verdicts WHERE batch_id=?`},
		{&st.Snapshots, `SELECT COUNT(*) FROM seasonal_snapshots WHERE batch_id=?`},
		{&st.PublishedSnaps, `SELECT COUNT(*) FROM seasonal_snapshots WHERE batch_id=? AND status='published'`},
	}
	for _, item := range queries {
		if err := s.DB.QueryRow(item.q, batchID).Scan(item.dst); err != nil {
			return nil, fmt.Errorf("store: batch stats: %w", err)
		}
	}
	return &st, nil
}
