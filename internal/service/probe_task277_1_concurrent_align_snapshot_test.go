package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
	"task277-shellband/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func mustShellBatch(t *testing.T, svc *Service, code string) *model.ShellBatch {
	t.Helper()
	b, err := svc.CreateBatch(code, "Pecten maximus")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddBands(b.ID, []*model.GrowthBand{
		{BandIndex: 0, StartPos: 0, EndPos: 10},
		{BandIndex: 1, StartPos: 10, EndPos: 20},
		{BandIndex: 2, StartPos: 20, EndPos: 30},
		{BandIndex: 3, StartPos: 30, EndPos: 40},
	}); err != nil {
		t.Fatal(err)
	}
	created, _, err := svc.AddSamples(b.ID, []*model.IsotopeSample{
		{SampleNo: 1, RawPos: 2, IsotopeValue: 1.0, Unit: "per mil", RecrystallScore: 0.1},
		{SampleNo: 2, RawPos: 15, IsotopeValue: 3.0, Unit: "per mil", RecrystallScore: 0.9},
		{SampleNo: 3, RawPos: 35, IsotopeValue: -1.0, Unit: "per mil", RecrystallScore: 0.2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Correct(b.ID, sample.DefaultCorrection()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Align(b.ID); err != nil {
		t.Fatal(err)
	}
	_ = created
	return b
}

func payloadSampleCount(t *testing.T, raw string) int {
	t.Helper()
	var payload model.SnapshotPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range payload.Bands {
		n += b.SampleCount
	}
	return n
}

func TestConcurrentAlignAndSnapshotIsolation(t *testing.T) {
	svc := newTestService(t)
	b := mustShellBatch(t, svc, "C-ALIGN-SNAP")
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if _, err := svc.Align(b.ID); err != nil {
					errCh <- fmt.Errorf("align %d: %w", i, err)
				}
				return
			}
			snap, err := svc.BuildSnapshot(b.ID, true)
			if err != nil {
				errCh <- fmt.Errorf("snapshot %d: %w", i, err)
				return
			}
			var payload model.SnapshotPayload
			if err := json.Unmarshal([]byte(snap.Payload), &payload); err != nil {
				errCh <- fmt.Errorf("snapshot %d decode: %w", i, err)
				return
			}
			n := 0
			for _, band := range payload.Bands {
				n += band.SampleCount
			}
			if n == 0 {
				errCh <- fmt.Errorf("snapshot %d version %d has zero sample counts", i, snap.Version)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
