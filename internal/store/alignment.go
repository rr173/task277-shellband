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
			// 同一样本已归属，视为幂等更新。
			return s.UpdateAlignment(a)
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

// DeleteStaleAlignments 删除批次中“不再属于给定 sample 集合”的旧归属关系。
// 用于对齐时的覆盖式更新：先写入新归属（CreateAlignment 幂等），再调用本方法
// 清理已被本轮对齐移出的旧记录，使任一时刻表内归属均完整、不为空。
func (s *Store) DeleteStaleAlignments(batchID int64, keepSamples map[int64]bool) error {
	if len(keepSamples) == 0 {
		// 本轮无任何归属：清空旧归属即正确结果（该批次确实无对齐样本）。
		if _, err := s.DB.Exec(`DELETE FROM alignments WHERE batch_id=?`, batchID); err != nil {
			return fmt.Errorf("store: delete stale alignments: %w", err)
		}
		return nil
	}
	// 构造占位符列表：batch_id 限定范围，NOT IN 剔除保留集合。
	ids := make([]interface{}, 0, len(keepSamples))
	placeholders := ""
	for sid := range keepSamples {
		if placeholders != "" {
			placeholders += ","
		}
		placeholders += "?"
		ids = append(ids, sid)
	}
	args := append([]interface{}{batchID}, ids...)
	q := fmt.Sprintf(
		`DELETE FROM alignments WHERE batch_id=? AND sample_id NOT IN (%s)`,
		placeholders)
	if _, err := s.DB.Exec(q, args...); err != nil {
		return fmt.Errorf("store: delete stale alignments: %w", err)
	}
	return nil
}
