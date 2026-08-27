package store

import (
	"testing"

	"task277-shellband/internal/model"
)

// mustSample 落库一条采样点，省略样板代码。
func mustSample(t *testing.T, s *Store, batchID int64, no int64, val float64) *model.IsotopeSample {
	t.Helper()
	sm, err := s.CreateSample(&model.IsotopeSample{
		BatchID: batchID, SampleNo: no, RawPos: float64(no),
		IsotopeValue: val, Unit: "per mil",
	})
	if err != nil {
		t.Fatal(err)
	}
	return sm
}

// TestListSamplesIsolatedAcrossBatches 回归：先列出甲批次采样、再列出乙批次采样，
// 甲那份返回结果不得被乙批次的列出覆盖。
// 旧实现用包级 sampleScratch 复用底层切片，第二次 ListSamples 会 copy 覆盖
// 第一次返回切片的内容，导致甲那份被改成乙批次的点。
func TestListSamplesIsolatedAcrossBatches(t *testing.T) {
	s, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	jia, err := s.CreateBatch("甲", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	yi, err := s.CreateBatch("乙", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	mustSample(t, s, jia.ID, 1, 1.1)
	mustSample(t, s, jia.ID, 2, 2.2)
	mustSample(t, s, jia.ID, 3, 3.3)
	mustSample(t, s, yi.ID, 1, 9.9)
	mustSample(t, s, yi.ID, 2, 8.8)
	// 关键顺序：先列甲（len=3，旧实现使其成为 sampleScratch 底层数组），
	// 再列乙（len=2 ≤ 甲的 cap=3，旧实现走 copy 覆盖分支，把甲返回切片
	// 前 2 个点改成乙的点）。修复后两次互不影响。

	jiaList, err := s.ListSamples(jia.ID)
	if err != nil {
		t.Fatal(err)
	}
	yiList, err := s.ListSamples(yi.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(jiaList) != 3 {
		t.Fatalf("甲 list len=%d want 3", len(jiaList))
	}
	if len(yiList) != 2 {
		t.Fatalf("乙 list len=%d want 2", len(yiList))
	}
	jiaWants := []float64{1.1, 2.2, 3.3}
	for i, sm := range jiaList {
		want := jiaWants[i]
		if sm.BatchID != jia.ID || sm.IsotopeValue != want {
			t.Fatalf("甲[%d] 被污染: batch=%d val=%v want batch=%d val=%v",
				i, sm.BatchID, sm.IsotopeValue, jia.ID, want)
		}
	}
	yiWants := []float64{9.9, 8.8}
	for i, sm := range yiList {
		want := yiWants[i]
		if sm.BatchID != yi.ID || sm.IsotopeValue != want {
			t.Fatalf("乙[%d] batch=%d val=%v want batch=%d val=%v",
				i, sm.BatchID, sm.IsotopeValue, yi.ID, want)
		}
	}
}
