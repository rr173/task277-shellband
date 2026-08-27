package store

import (
	"path/filepath"
	"testing"
	"time"

	"task277-shellband/internal/model"
)

// TestListCorrectionsDoesNotLeakConnection 回归 ListCorrections 不再泄漏 rows。
// 连接池设为单连接（SetMaxOpenConns(1)）：若 ListCorrections 返回后仍占用唯一
// 连接，后续 GlobalStats 的 QueryRow.Scan 会永久阻塞，测试在超时内即失败。
func TestListCorrectionsDoesNotLeakConnection(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "corr.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	batch, err := st.CreateBatch("CORR-LEAK", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	sample, err := st.CreateSample(&model.IsotopeSample{
		BatchID: batch.ID, SampleNo: 1, RawPos: 2, IsotopeValue: 1.0,
		Unit: "per mil", RecrystallScore: 0.1,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if err := st.CreateCorrection(&model.PositionCorrection{
			BatchID:      batch.ID,
			SampleID:     sample.ID,
			RawPos:       2,
			CorrectedPos: 2,
			Method:       "linear-shrinkage",
			AppliedAt:    time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := st.ListCorrections(batch.ID); err != nil {
		t.Fatal(err)
	}

	// 若连接被泄漏，此调用会阻塞直至测试超时。
	done := make(chan error, 1)
	go func() {
		_, err := st.GlobalStats()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("global stats after list corrections: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("GlobalStats blocked: ListCorrections leaked the pooled connection")
	}
}
