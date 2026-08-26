package store

import (
	"database/sql"
	"fmt"
	"time"

	"task277-shellband/internal/model"
)

// UpsertVerdict 写入（或覆盖）对某采样点的污染裁决。
func (s *Store) UpsertVerdict(v *model.PollutionVerdict) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// 先删除同批次同采样点的旧裁决，保证一份最新裁决。
	if _, err := s.DB.Exec(`DELETE FROM pollution_verdicts WHERE batch_id=? AND sample_id=?`,
		v.BatchID, v.SampleID); err != nil {
		return fmt.Errorf("store: delete old verdict: %w", err)
	}
	_, err := s.DB.Exec(
		`INSERT INTO pollution_verdicts(batch_id, sample_id, verdict, reason, reviewer, at)
		 VALUES(?,?,?,?,?,?)`,
		v.BatchID, v.SampleID, v.Verdict, v.Reason, v.Reviewer, now)
	if err != nil {
		return fmt.Errorf("store: upsert verdict: %w", err)
	}
	return nil
}

// ListVerdicts 列出批次全部污染裁决。
func (s *Store) ListVerdicts(batchID int64) ([]*model.PollutionVerdict, error) {
	rows, err := s.DB.Query(
		`SELECT id, batch_id, sample_id, verdict, reason, reviewer, at
		 FROM pollution_verdicts WHERE batch_id=? ORDER BY at ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("store: list verdicts: %w", err)
	}
	defer rows.Close()
	var out []*model.PollutionVerdict
	for rows.Next() {
		var (
			id       int64
			batchID  int64
			sampleID int64
			verdict  string
			reason   string
			reviewer string
			at       string
		)
		if err := rows.Scan(&id, &batchID, &sampleID, &verdict, &reason, &reviewer, &at); err != nil {
			return nil, fmt.Errorf("store: scan verdict: %w", err)
		}
		out = append(out, &model.PollutionVerdict{
			ID:       id,
			BatchID:  batchID,
			SampleID: sampleID,
			Verdict:  verdict,
			Reason:   reason,
			Reviewer: reviewer,
			At:       mustParseTime(at),
		})
	}
	return out, rows.Err()
}

// GetVerdict 读取某采样点的最新裁决（无则返回 nil, nil）。
func (s *Store) GetVerdict(batchID, sampleID int64) (*model.PollutionVerdict, error) {
	row := s.DB.QueryRow(
		`SELECT id, batch_id, sample_id, verdict, reason, reviewer, at
		 FROM pollution_verdicts WHERE batch_id=? AND sample_id=? ORDER BY id DESC LIMIT 1`,
		batchID, sampleID)
	v, err := scanVerdict(row)
	if err == ErrVerdictNotFound {
		return nil, nil
	}
	return v, err
}

var ErrVerdictNotFound = fmt.Errorf("verdict not found")

// errNoRows 用于 scanVerdict 识别“无记录”。
var errNoRows = sql.ErrNoRows

func scanVerdict(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.PollutionVerdict, error) {
	var (
		id       int64
		batchID  int64
		sampleID int64
		verdict  string
		reason   string
		reviewer string
		at       string
	)
	if err := scanner.Scan(&id, &batchID, &sampleID, &verdict, &reason, &reviewer, &at); err != nil {
		if err == errNoRows {
			return nil, ErrVerdictNotFound
		}
		return nil, fmt.Errorf("store: scan verdict: %w", err)
	}
	return &model.PollutionVerdict{
		ID:       id,
		BatchID:  batchID,
		SampleID: sampleID,
		Verdict:  verdict,
		Reason:   reason,
		Reviewer: reviewer,
		At:       mustParseTime(at),
	}, nil
}
