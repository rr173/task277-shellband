package store

import (
	"database/sql"
	"fmt"
	"time"

	"task277-shellband/internal/model"
)

// NextSnapshotVersion 返回批次下一个快照版本号（当前最大版本 + 1）。
func (s *Store) NextSnapshotVersion(batchID int64) (int, error) {
	var cur int
	if err := s.DB.QueryRow(
		`SELECT COALESCE(MAX(version),0) FROM seasonal_snapshots WHERE batch_id=?`, batchID).Scan(&cur); err != nil {
		return 0, fmt.Errorf("store: next snapshot version: %w", err)
	}
	return cur + 1, nil
}

// SupersedePublished 将批次已发布的快照置为“替代”。
func (s *Store) SupersedePublished(batchID int64) error {
	if _, err := s.DB.Exec(
		`UPDATE seasonal_snapshots SET status=? WHERE batch_id=?`,
		string(model.SnapshotSuperseded), batchID); err != nil {
		return fmt.Errorf("store: supersede published: %w", err)
	}
	return nil
}

// CreateSnapshot 写入季节快照（草稿或发布）。
func (s *Store) CreateSnapshot(snap *model.SeasonalSnapshot) (*model.SeasonalSnapshot, error) {
	if snap.BatchID <= 0 {
		return nil, model.NewDomainError("BAD_BATCH", "batch id required", nil)
	}
	if snap.Payload == "" {
		return nil, model.NewDomainError("BAD_PAYLOAD", "snapshot payload empty", nil)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sealed := 0
	if snap.Sealed {
		sealed = 1
	}
	res, err := s.DB.Exec(
		`INSERT INTO seasonal_snapshots(batch_id, version, status, sealed, payload, created_at)
		 VALUES(?,?,?,?,?,?)`,
		snap.BatchID, snap.Version, string(snap.Status), sealed, snap.Payload, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.NewDomainError("DUP_VERSION", "snapshot version conflict", nil)
		}
		return nil, fmt.Errorf("store: create snapshot: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetSnapshot(id)
}

// GetSnapshot 按 ID 读取快照（payload 一并返回）。
func (s *Store) GetSnapshot(id int64) (*model.SeasonalSnapshot, error) {
	row := s.DB.QueryRow(
		`SELECT id, batch_id, version, status, sealed, payload, created_at FROM seasonal_snapshots WHERE id=?`, id)
	return scanSnapshot(row)
}

// ListSnapshots 按版本升序列出批次全部快照。
func (s *Store) ListSnapshots(batchID int64) ([]*model.SeasonalSnapshot, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, version, status, sealed, payload, created_at
		 FROM seasonal_snapshots WHERE batch_id=? ORDER BY version ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list snapshots: %w", err)
	}
	defer rows.Close()
	var out []*model.SeasonalSnapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// ListPublishedSnapshots 列出批次已发布快照。
func (s *Store) ListPublishedSnapshots(batchID int64) ([]*model.SeasonalSnapshot, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, version, status, sealed, payload, created_at
		 FROM seasonal_snapshots WHERE batch_id=? AND status=? ORDER BY version ASC`,
		batchID, string(model.SnapshotPublished))
	if err != nil {
		return nil, fmt.Errorf("store: list published snapshots: %w", err)
	}
	defer rows.Close()
	var out []*model.SeasonalSnapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func scanSnapshot(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SeasonalSnapshot, error) {
	var (
		id        int64
		batchID   int64
		version   int
		status    string
		sealed    int
		payload   string
		createdAt string
	)
	if err := scanner.Scan(&id, &batchID, &version, &status, &sealed, &payload, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSnapshotNotFound
		}
		return nil, fmt.Errorf("store: scan snapshot: %w", err)
	}
	return &model.SeasonalSnapshot{
		ID:        id,
		BatchID:   batchID,
		Version:   version,
		Status:    model.SnapshotStatus(status),
		Sealed:    sealed != 0,
		Payload:   payload,
		CreatedAt: mustParseTime(createdAt),
	}, nil
}
