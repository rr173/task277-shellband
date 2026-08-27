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

func TestCreateBatchDuplicateCodeConflicts(t *testing.T) {
	srv := newServer(t)
	body := strings.NewReader(`{"code":"HTTP-DUP","species":"Pecten"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/batches", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create=%d body=%s", rec.Code, rec.Body.String())
	}
	// 重复编码必须按冲突（409）返回，而非内部错误（500）。
	req = httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(`{"code":"HTTP-DUP","species":"Pecten"}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate code: want %d, got %d body=%s", http.StatusConflict, rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] != "CONFLICT" {
		t.Fatalf("error code: want CONFLICT, got %v", got["error"])
	}
}
