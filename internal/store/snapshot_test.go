package store

import (
	"path/filepath"
	"testing"

	"task277-shellband/internal/model"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestListSnapshotsSeesAllVersions 回归：同一批次连续发布两个版本的快照后，
// ListSnapshots 必须能看到全部版本，而非只停留在首版。
//
// 此前 ListSnapshots 把结果缓存在 Store.snapCache 且永不失效，第二次发布后
// 仍返回首版缓存，导致“列快照只看得到第一版”。
func TestListSnapshotsSeesAllVersions(t *testing.T) {
	st := newStore(t)
	b, err := st.CreateBatch("SNAP-MULTI", "Pecten")
	if err != nil {
		t.Fatal(err)
	}

	// 发布第一版。
	v1, err := st.NextSnapshotVersion(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSnapshot(&model.SeasonalSnapshot{
		BatchID: b.ID, Version: v1, Status: model.SnapshotPublished, Payload: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	if got := mustListLen(t, st, b.ID); got != 1 {
		t.Fatalf("after first publish: got %d snapshots, want 1", got)
	}

	// 第二次发布：先替代已发布者，再写入新版本。
	if err := st.SupersedePublished(b.ID); err != nil {
		t.Fatal(err)
	}
	v2, err := st.NextSnapshotVersion(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSnapshot(&model.SeasonalSnapshot{
		BatchID: b.ID, Version: v2, Status: model.SnapshotPublished, Payload: "v2",
	}); err != nil {
		t.Fatal(err)
	}

	snaps, err := st.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("after second publish: got %d snapshots, want 2 (versions: %v)",
			len(snaps), versions(snaps))
	}
	if snaps[0].Version != v1 || snaps[1].Version != v2 {
		t.Fatalf("unexpected order: got versions %v, want [%d, %d]",
			versions(snaps), v1, v2)
	}
}

// TestListSnapshotsReflectsStatusChange 回归：发布态改写（supersede）后，
// ListSnapshots 必须反映最新的 status，而非缓存里的旧值。
func TestListSnapshotsReflectsStatusChange(t *testing.T) {
	st := newStore(t)
	b, err := st.CreateBatch("SNAP-STATUS", "Pecten")
	if err != nil {
		t.Fatal(err)
	}
	v1, err := st.NextSnapshotVersion(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	created, err := st.CreateSnapshot(&model.SeasonalSnapshot{
		BatchID: b.ID, Version: v1, Status: model.SnapshotPublished, Payload: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 触发缓存填充（旧实现下首版为 published）。
	if got := mustListLen(t, st, b.ID); got != 1 {
		t.Fatalf("after first publish: got %d, want 1", got)
	}
	if err := st.SupersedePublished(b.ID); err != nil {
		t.Fatal(err)
	}
	snaps, err := st.ListSnapshots(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].ID != created.ID {
		t.Fatalf("unexpected snaps: %+v", snaps)
	}
	if snaps[0].Status != model.SnapshotSuperseded {
		t.Fatalf("status not refreshed: got %q, want %q",
			snaps[0].Status, model.SnapshotSuperseded)
	}
}

func mustListLen(t *testing.T, st *Store, batchID int64) int {
	t.Helper()
	snaps, err := st.ListSnapshots(batchID)
	if err != nil {
		t.Fatal(err)
	}
	return len(snaps)
}

func versions(snaps []*model.SeasonalSnapshot) []int {
	out := make([]int, len(snaps))
	for i, s := range snaps {
		out[i] = s.Version
	}
	return out
}
