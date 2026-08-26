package store

import (
	"database/sql"
	"fmt"

	"task277-shellband/internal/model"
)

// CreateAnchor 新增年代锚点。同批次内 position 唯一（冲突即报错）。
func (s *Store) CreateAnchor(a *model.AgeAnchor) (*model.AgeAnchor, error) {
	if a.BatchID <= 0 {
		return nil, model.NewDomainError("BAD_BATCH", "batch id required", nil)
	}
	res, err := s.DB.Exec(
		`INSERT INTO age_anchors(batch_id, position, age_year, source) VALUES(?,?,?,?)`,
		a.BatchID, a.Position, a.AgeYear, a.Source)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.ErrAnchorConflict
		}
		return nil, fmt.Errorf("store: create anchor: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetAnchor(id)
}

// GetAnchor 按 ID 读取锚点。
func (s *Store) GetAnchor(id int64) (*model.AgeAnchor, error) {
	row := s.DB.QueryRow(
		`SELECT id, batch_id, position, age_year, source FROM age_anchors WHERE id=?`, id)
	return scanAnchor(row)
}

// ListAnchors 按位置升序列出批次的全部年代锚点。
func (s *Store) ListAnchors(batchID int64) ([]*model.AgeAnchor, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, position, age_year, source FROM age_anchors WHERE batch_id=? ORDER BY position ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list anchors: %w", err)
	}
	defer rows.Close()
	var out []*model.AgeAnchor
	for rows.Next() {
		a, err := scanAnchor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanAnchor(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.AgeAnchor, error) {
	var (
		id       int64
		batchID  int64
		position float64
		ageYear  float64
		source   string
	)
	if err := scanner.Scan(&id, &batchID, &position, &ageYear, &source); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrAnchorNotFound
		}
		return nil, fmt.Errorf("store: scan anchor: %w", err)
	}
	return &model.AgeAnchor{
		ID:       id,
		BatchID:  batchID,
		Position: position,
		AgeYear:  ageYear,
		Source:   source,
	}, nil
}
