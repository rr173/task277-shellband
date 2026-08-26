// Package store 负责 SQLite 持久化：建表迁移与各实体的增删改查。
// 所有写操作在单写连接上串行，由上层保证同一切片对齐的串行语义。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store 封装 SQLite 连接与所有持久化操作。
type Store struct {
	DB *sql.DB
}

// Open 打开（或创建）SQLite 数据库文件并完成迁移。
// path 为空时使用内存库，便于 --smoke-test 自检。
func Open(path string) (*Store, error) {
	dsn := path
	if dsn == "" {
		dsn = ":memory:"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}
	// SQLite 单写者：限制单连接，避免并发写导致的 busy 错误，
	// 同时与业务层“采样可并行、对齐串行”的语义对齐。
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping sqlite: %w", err)
	}
	s := &Store{DB: db}
	if err := s.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭底层连接。
func (s *Store) Close() error {
	if s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// Migrate 创建全部表与索引（幂等，可重复调用）。
func (s *Store) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS shell_batches (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			code       TEXT NOT NULL UNIQUE,
			species    TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL,
			created_at TEXT NOT NULL,
			sealed_at  TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS growth_bands (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id   INTEGER NOT NULL,
			band_index INTEGER NOT NULL,
			start_pos  REAL NOT NULL,
			end_pos    REAL NOT NULL,
			kind       TEXT NOT NULL,
			note       TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS isotope_samples (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id         INTEGER NOT NULL,
			sample_no        INTEGER NOT NULL,
			raw_pos          REAL NOT NULL,
			corrected_pos    REAL NOT NULL DEFAULT 0,
			isotope_value    REAL NOT NULL,
			unit             TEXT NOT NULL DEFAULT '',
			recrystall_score REAL NOT NULL DEFAULT 0,
			status           TEXT NOT NULL,
			band_id          INTEGER,
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			UNIQUE(batch_id, sample_no)
		)`,
		`CREATE TABLE IF NOT EXISTS age_anchors (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			position REAL NOT NULL,
			age_year REAL NOT NULL,
			source   TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			UNIQUE(batch_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS position_corrections (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id      INTEGER NOT NULL,
			sample_id     INTEGER NOT NULL,
			raw_pos       REAL NOT NULL,
			corrected_pos REAL NOT NULL,
			method        TEXT NOT NULL DEFAULT '',
			applied_at    TEXT NOT NULL,
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			FOREIGN KEY(sample_id) REFERENCES isotope_samples(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS alignments (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			band_id  INTEGER NOT NULL,
			sample_id INTEGER NOT NULL,
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			UNIQUE(batch_id, sample_id)
		)`,
		`CREATE TABLE IF NOT EXISTS pollution_verdicts (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id  INTEGER NOT NULL,
			sample_id INTEGER NOT NULL,
			verdict   TEXT NOT NULL,
			reason    TEXT NOT NULL DEFAULT '',
			reviewer  TEXT NOT NULL DEFAULT '',
			at        TEXT NOT NULL,
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			UNIQUE(batch_id, sample_id)
		)`,
		`CREATE TABLE IF NOT EXISTS seasonal_snapshots (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id   INTEGER NOT NULL,
			version    INTEGER NOT NULL,
			status     TEXT NOT NULL,
			sealed     INTEGER NOT NULL DEFAULT 0,
			payload    TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			FOREIGN KEY(batch_id) REFERENCES shell_batches(id) ON DELETE CASCADE,
			UNIQUE(batch_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bands_batch ON growth_bands(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_batch ON isotope_samples(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_band ON isotope_samples(batch_id, band_id)`,
		`CREATE INDEX IF NOT EXISTS idx_anchors_batch ON age_anchors(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_align_batch ON alignments(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_verdict_batch ON pollution_verdicts(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snap_batch ON seasonal_snapshots(batch_id)`,
	}
	for _, st := range stmts {
		if _, err := s.DB.Exec(st); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}
