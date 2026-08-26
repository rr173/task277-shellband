package store

import (
	"database/sql"
	"fmt"

	"task277-shellband/internal/model"
)

// CreateBand 新增一条生长带边界。band_index 应递増排布。
func (s *Store) CreateBand(b *model.GrowthBand) (*model.GrowthBand, error) {
	if b.BatchID <= 0 {
		return nil, model.NewDomainError("BAD_BATCH", "batch id required", nil)
	}
	if b.EndPos < b.StartPos {
		return nil, model.NewDomainError("BAD_RANGE", "band end_pos < start_pos", nil)
	}
	res, err := s.DB.Exec(
		`INSERT INTO growth_bands(batch_id, band_index, start_pos, end_pos, kind, note)
		 VALUES(?,?,?,?,?,?)`,
		b.BatchID, b.BandIndex, b.StartPos, b.EndPos, string(model.BandCandidate), b.Note)
	if err != nil {
		return nil, fmt.Errorf("store: create band: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetBand(id)
}

// GetBand 按 ID 读取生长带。
func (s *Store) GetBand(id int64) (*model.GrowthBand, error) {
	row := s.DB.QueryRow(
		`SELECT id, batch_id, band_index, start_pos, end_pos, kind, note FROM growth_bands WHERE id=?`, id)
	return scanBand(row)
}

// ListBands 按索引顺序列出批次的全部生长带。
func (s *Store) ListBands(batchID int64) ([]*model.GrowthBand, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, band_index, start_pos, end_pos, kind, note
		 FROM growth_bands WHERE batch_id=? ORDER BY band_index ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list bands: %w", err)
	}
	defer rows.Close()
	var out []*model.GrowthBand
	for rows.Next() {
		b, err := scanBand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBandKind 更新生长带判定种类与备注（对齐/诊断阶段写入）。
func (s *Store) UpdateBandKind(id int64, kind model.BandKind, note string) error {
	res, err := s.DB.Exec(`UPDATE growth_bands SET kind=?, note=? WHERE id=?`, string(kind), note, id)
	if err != nil {
		return fmt.Errorf("store: update band kind: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrBandNotFound
	}
	return nil
}

// DeleteBands 删除批次的全部生长带（重对齐前清空用）。
func (s *Store) DeleteBands(batchID int64) error {
	if _, err := s.DB.Exec(`DELETE FROM growth_bands WHERE batch_id=?`, batchID); err != nil {
		return fmt.Errorf("store: delete bands: %w", err)
	}
	return nil
}

func scanBand(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.GrowthBand, error) {
	var (
		id        int64
		batchID   int64
		bandIndex int
		startPos  float64
		endPos    float64
		kind      string
		note      string
	)
	if err := scanner.Scan(&id, &batchID, &bandIndex, &startPos, &endPos, &kind, &note); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrBandNotFound
		}
		return nil, fmt.Errorf("store: scan band: %w", err)
	}
	return &model.GrowthBand{
		ID:        id,
		BatchID:   batchID,
		BandIndex: bandIndex,
		StartPos:  startPos,
		EndPos:    endPos,
		Kind:      model.BandKind(kind),
		Note:      note,
	}, nil
}
