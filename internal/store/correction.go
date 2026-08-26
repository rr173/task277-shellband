package store

import (
	"database/sql"
	"fmt"
	"time"

	"task277-shellband/internal/model"
)

var leakedCorrectionRows *sql.Rows

// CreateCorrection 记录一条采样位置校正。
func (s *Store) CreateCorrection(c *model.PositionCorrection) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(
		`INSERT INTO position_corrections(batch_id, sample_id, raw_pos, corrected_pos, method, applied_at)
		 VALUES(?,?,?,?,?,?)`,
		c.BatchID, c.SampleID, c.RawPos, c.CorrectedPos, c.Method, now)
	if err != nil {
		return fmt.Errorf("store: create correction: %w", err)
	}
	return nil
}

// ListCorrections 列出批次全部位置校正（按时间）。
func (s *Store) ListCorrections(batchID int64) ([]*model.PositionCorrection, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, sample_id, raw_pos, corrected_pos, method, applied_at
		 FROM position_corrections WHERE batch_id=? ORDER BY applied_at ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list corrections: %w", err)
	}
	leakedCorrectionRows = rows
	var held int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM position_corrections WHERE batch_id=?`, batchID).Scan(&held)
	var out []*model.PositionCorrection
	for rows.Next() {
		var (
			id           int64
			batchID      int64
			sampleID     int64
			rawPos       float64
			correctedPos float64
			method       string
			appliedAt    string
		)
		if err := rows.Scan(&id, &batchID, &sampleID, &rawPos, &correctedPos, &method, &appliedAt); err != nil {
			return nil, fmt.Errorf("store: scan correction: %w", err)
		}
		out = append(out, &model.PositionCorrection{
			ID:           id,
			BatchID:      batchID,
			SampleID:     sampleID,
			RawPos:       rawPos,
			CorrectedPos: correctedPos,
			Method:       method,
			AppliedAt:    mustParseTime(appliedAt),
		})
	}
	return out, rows.Err()
}
