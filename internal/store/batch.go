package store

import (
	"database/sql"
	"fmt"
	"time"

	"task277-shellband/internal/model"
)

// CreateBatch 创建贝壳批次（初始状态 receiving）。编码须唯一。
func (s *Store) CreateBatch(code, species string) (*model.ShellBatch, error) {
	if code == "" {
		return nil, model.NewDomainError("BAD_CODE", "batch code is empty", nil)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.DB.Exec(
		`INSERT INTO shell_batches(code, species, status, created_at) VALUES(?,?,?,?)`,
		code, species, string(model.BatchReceiving), now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.ErrDuplicateCode
		}
		return nil, fmt.Errorf("store: create batch: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetBatch(id)
}

// GetBatch 按 ID 读取批次。
func (s *Store) GetBatch(id int64) (*model.ShellBatch, error) {
	row := s.DB.QueryRow(
		`SELECT id, code, species, status, created_at, sealed_at FROM shell_batches WHERE id=?`, id)
	return scanBatch(row)
}

// GetBatchByCode 按编码读取批次。
func (s *Store) GetBatchByCode(code string) (*model.ShellBatch, error) {
	row := s.DB.QueryRow(
		`SELECT id, code, species, status, created_at, sealed_at FROM shell_batches WHERE code=?`, code)
	return scanBatch(row)
}

// ListBatches 列出全部批次（按创建时间升序）。
func (s *Store) ListBatches() ([]*model.ShellBatch, error) {
	rows, err := s.DB.Query(
		`SELECT id, code, species, status, created_at, sealed_at FROM shell_batches ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list batches: %w", err)
	}
	defer rows.Close()
	var out []*model.ShellBatch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBatchStatus 更新批次状态，调用方须先通过状态机校验。
func (s *Store) UpdateBatchStatus(id int64, st model.BatchStatus, sealedAt *time.Time) error {
	var sealedSQL interface{}
	if sealedAt != nil {
		sealedSQL = sealedAt.UTC().Format(time.RFC3339)
	} else {
		sealedSQL = nil
	}
	res, err := s.DB.Exec(`UPDATE shell_batches SET status=?, sealed_at=? WHERE id=?`,
		string(st), sealedSQL, id)
	if err != nil {
		return fmt.Errorf("store: update batch status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrBatchNotFound
	}
	return nil
}

// SetBatchSpecies 设置物种信息（仅接收中状态允许）。
func (s *Store) SetBatchSpecies(id int64, species string) error {
	res, err := s.DB.Exec(`UPDATE shell_batches SET species=? WHERE id=?`, species, id)
	if err != nil {
		return fmt.Errorf("store: set species: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrBatchNotFound
	}
	return nil
}

func scanBatch(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.ShellBatch, error) {
	var (
		id        int64
		code      string
		species   string
		status    string
		createdAt string
		sealedAt  sql.NullString
	)
	if err := scanner.Scan(&id, &code, &species, &status, &createdAt, &sealedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrBatchNotFound
		}
		return nil, fmt.Errorf("store: scan batch: %w", err)
	}
	b := &model.ShellBatch{
		ID:        id,
		Code:      code,
		Species:   species,
		Status:    model.BatchStatus(status),
		CreatedAt: mustParseTime(createdAt),
	}
	if sealedAt.Valid {
		t := mustParseTime(sealedAt.String)
		b.SealedAt = &t
	}
	return b, nil
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// isUniqueViolation 粗略判定是否为唯一约束冲突（modernc sqlite 错误含关键字）。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "UNIQUE") || contains(msg, "constraint failed")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
