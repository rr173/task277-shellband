package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task277-shellband/internal/service"
	"task277-shellband/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewServer(service.New(st)).Handler()
}

func TestDuplicateBatchCodeMapsConflict(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"code":"DUP-1","species":"Pecten"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"code":"DUP-1","species":"Pecten"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] != "CONFLICT" {
		t.Fatalf("error=%q body=%v", got["error"], got)
	}
}
