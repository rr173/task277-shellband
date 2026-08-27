package store

import (
	"fmt"
	"time"

	"task277-shellband/internal/model"
)

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
//
// 必须在返回前关闭 rows：连接池仅设为单连接（SetMaxOpenConns(1)），
// 未关闭的 *sql.Rows 会一直占用唯一连接，导致后续任何查询（含全库
// 统计 GlobalStats）永久阻塞。
func (s *Store) ListCorrections(batchID int64) ([]*model.PositionCorrection, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, sample_id, raw_pos, corrected_pos, method, applied_at
		 FROM position_corrections WHERE batch_id=? ORDER BY applied_at ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list corrections: %w", err)
	}
	defer rows.Close()
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
