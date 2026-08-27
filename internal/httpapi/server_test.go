package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"errors"

	"task277-shellband/internal/model"
	"task277-shellband/internal/service"
	"task277-shellband/internal/store"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewServer(service.New(st))
}

func TestHealthAndCreateBatch(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health=%d", rec.Code)
	}

	body := strings.NewReader(`{"code":"HTTP-1","species":"Pecten"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "HTTP-1" {
		t.Fatalf("got %+v", got)
	}
}

func TestStatsEmpty(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats=%d", rec.Code)
	}
}

func TestGetBatchNotFound(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/batches/99999", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] != "NOT_FOUND" {
		t.Fatalf("error=%v, want NOT_FOUND", got["error"])
	}

	// 直接断言 service 层哨兵，确保调用方可按缺失批次处理。
	if _, err := srv.svc.GetBatch(99999); !errors.Is(err, model.ErrBatchNotFound) {
		t.Fatalf("GetBatch err=%v, want ErrBatchNotFound", err)
	}
}
