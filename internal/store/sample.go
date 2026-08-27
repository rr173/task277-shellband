package store

import (
	"database/sql"
	"fmt"

	"task277-shellband/internal/model"
)

// CreateSample 新增一条氧同位素采样点。sample_no 在批次内唯一（幂等编号）。
func (s *Store) CreateSample(sample *model.IsotopeSample) (*model.IsotopeSample, error) {
	if sample.BatchID <= 0 {
		return nil, model.NewDomainError("BAD_BATCH", "batch id required", nil)
	}
	if sample.Unit == "" {
		return nil, model.ErrMissingUnit
	}
	res, err := s.DB.Exec(
		`INSERT INTO isotope_samples(batch_id, sample_no, raw_pos, corrected_pos, isotope_value, unit, recrystall_score, status, band_id)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		sample.BatchID, sample.SampleNo, sample.RawPos, sample.CorrectedPos,
		sample.IsotopeValue, sample.Unit, sample.RecrystallScore, string(model.SampleRaw), nil)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.ErrSampleNoConflict
		}
		return nil, fmt.Errorf("store: create sample: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetSample(id)
}

// GetSample 按 ID 读取采样点。
func (s *Store) GetSample(id int64) (*model.IsotopeSample, error) {
	row := s.DB.QueryRow(
		`SELECT id, batch_id, sample_no, raw_pos, corrected_pos, isotope_value, unit, recrystall_score, status, band_id
		 FROM isotope_samples WHERE id=?`, id)
	return scanSample(row)
}

// ListSamples 按编号顺序列出批次的全部采样点。
// 每次返回独立的切片，多次调用互不影响（不复用共享缓冲，否则后一次
// 列出会覆盖前一次返回的切片内容）。
func (s *Store) ListSamples(batchID int64) ([]*model.IsotopeSample, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, sample_no, raw_pos, corrected_pos, isotope_value, unit, recrystall_score, status, band_id
		 FROM isotope_samples WHERE batch_id=? ORDER BY sample_no ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list samples: %w", err)
	}
	defer rows.Close()
	var out []*model.IsotopeSample
	for rows.Next() {
		sm, err := scanSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateSampleStatus 更新采样点状态与归属带。
func (s *Store) UpdateSampleStatus(id int64, st model.SampleStatus, bandID *int64) error {
	var bandSQL interface{}
	if bandID != nil {
		bandSQL = *bandID
	} else {
		bandSQL = nil
	}
	res, err := s.DB.Exec(`UPDATE isotope_samples SET status=?, band_id=? WHERE id=?`,
		string(st), bandSQL, id)
	if err != nil {
		return fmt.Errorf("store: update sample status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrSampleNotFound
	}
	return nil
}

// SetCorrectedPos 写入校正后位置（位置校正阶段）。
func (s *Store) SetCorrectedPos(id int64, corrected float64) error {
	res, err := s.DB.Exec(`UPDATE isotope_samples SET corrected_pos=? WHERE id=?`, corrected, id)
	if err != nil {
		return fmt.Errorf("store: set corrected pos: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrSampleNotFound
	}
	return nil
}

// CountSamples 返回批次采样点总数。
func (s *Store) CountSamples(batchID int64) (int, error) {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM isotope_samples WHERE batch_id=?`, batchID).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count samples: %w", err)
	}
	return n, nil
}

func scanSample(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.IsotopeSample, error) {
	var (
		id              int64
		batchID         int64
		sampleNo        int64
		rawPos          float64
		correctedPos    float64
		isotopeValue    float64
		unit            string
		recrystallScore float64
		status          string
		bandID          sql.NullInt64
	)
	if err := scanner.Scan(&id, &batchID, &sampleNo, &rawPos, &correctedPos, &isotopeValue, &unit, &recrystallScore, &status, &bandID); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSampleNotFound
		}
		return nil, fmt.Errorf("store: scan sample: %w", err)
	}
	sm := &model.IsotopeSample{
		ID:              id,
		BatchID:         batchID,
		SampleNo:        sampleNo,
		RawPos:          rawPos,
		CorrectedPos:    correctedPos,
		IsotopeValue:    isotopeValue,
		Unit:            unit,
		RecrystallScore: recrystallScore,
		Status:          model.SampleStatus(status),
	}
	if bandID.Valid {
		v := bandID.Int64
		sm.BandID = &v
	}
	return sm, nil
}
