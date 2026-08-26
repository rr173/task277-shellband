package store

import (
	"fmt"

	"task277-shellband/internal/model"
)

// CreateAlignment 记录采样点到生长带的归属。
func (s *Store) CreateAlignment(a *model.Alignment) error {
	_, err := s.DB.Exec(
		`INSERT INTO alignments(batch_id, band_id, sample_id) VALUES(?,?,?)`,
		a.BatchID, a.BandID, a.SampleID)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("store: duplicate alignment: %v", err)
		}
		return fmt.Errorf("store: create alignment: %w", err)
	}
	return nil
}

// UpdateAlignment 更新采样点归属的生长带。
func (s *Store) UpdateAlignment(a *model.Alignment) error {
	res, err := s.DB.Exec(
		`UPDATE alignments SET band_id=? WHERE batch_id=? AND sample_id=?`,
		a.BandID, a.BatchID, a.SampleID)
	if err != nil {
		return fmt.Errorf("store: update alignment: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: alignment not found")
	}
	return nil
}

// ListAlignments 列出批次全部归属关系。
func (s *Store) ListAlignments(batchID int64) ([]*model.Alignment, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, band_id, sample_id FROM alignments WHERE batch_id=? ORDER BY band_id ASC, sample_id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list alignments: %w", err)
	}
	defer rows.Close()
	var out []*model.Alignment
	for rows.Next() {
		var (
			id       int64
			batchID  int64
			bandID   int64
			sampleID int64
		)
		if err := rows.Scan(&id, &batchID, &bandID, &sampleID); err != nil {
			return nil, fmt.Errorf("store: scan alignment: %w", err)
		}
		out = append(out, &model.Alignment{ID: id, BatchID: batchID, BandID: bandID, SampleID: sampleID})
	}
	return out, rows.Err()
}

// DeleteAlignments 删除批次全部归属关系（重对齐前清空）。
func (s *Store) DeleteAlignments(batchID int64) error {
	if _, err := s.DB.Exec(`DELETE FROM alignments WHERE batch_id=?`, batchID); err != nil {
		return fmt.Errorf("store: delete alignments: %w", err)
	}
	return nil
}
