// Package httpapi 暴露 REST 风格的 HTTP 接口，承载贝壳同位素季节生长带对齐服务。
// 仅依赖标准库 net/http（含 Go 1.22+ 的 method+path 路由模式），无外部 Web 框架。
package httpapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"task277-shellband/internal/service"
)

// Server 持有业务 Service 与路由。
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// NewServer 构造 HTTP 服务并注册全部路由。
func NewServer(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.register()
	return s
}

// Handler 返回可传入 http.ListenAndServe 的 http.Handler。
func (s *Server) Handler() http.Handler {
	return s.mux
}

// register 注册全部 /api 路由（≥20 个接口）。
func (s *Server) register() {
	m := s.mux
	m.HandleFunc("GET /healthz", s.handleHealth)
	m.HandleFunc("GET /api/stats", s.handleGlobalStats)

	m.HandleFunc("POST /api/batches", s.handleCreateBatch)
	m.HandleFunc("GET /api/batches", s.handleListBatches)
	m.HandleFunc("GET /api/batches/{id}", s.handleGetBatch)
	m.HandleFunc("PATCH /api/batches/{id}/species", s.handleSetSpecies)
	m.HandleFunc("POST /api/batches/{id}/bands", s.handleAddBands)
	m.HandleFunc("GET /api/batches/{id}/bands", s.handleListBands)
	m.HandleFunc("POST /api/batches/{id}/samples", s.handleAddSamples)
	m.HandleFunc("GET /api/batches/{id}/samples", s.handleListSamples)
	m.HandleFunc("POST /api/batches/{id}/correct", s.handleCorrect)
	m.HandleFunc("GET /api/batches/{id}/corrections", s.handleListCorrections)
	m.HandleFunc("POST /api/batches/{id}/align", s.handleAlign)
	m.HandleFunc("GET /api/batches/{id}/alignments", s.handleListAlignments)
	m.HandleFunc("POST /api/batches/{id}/diagnose", s.handleDiagnose)
	m.HandleFunc("POST /api/batches/{id}/verdicts", s.handleRecordVerdicts)
	m.HandleFunc("GET /api/batches/{id}/verdicts", s.handleListVerdicts)
	m.HandleFunc("POST /api/batches/{id}/snapshots", s.handleBuildSnapshot)
	m.HandleFunc("GET /api/batches/{id}/snapshots", s.handleListSnapshots)
	m.HandleFunc("GET /api/batches/{id}/snapshots/{sid}", s.handleGetSnapshot)
	m.HandleFunc("POST /api/batches/{id}/publish", s.handlePublishSnapshot)
	m.HandleFunc("POST /api/batches/{id}/status", s.handleTransitionStatus)
	m.HandleFunc("POST /api/batches/{id}/seal", s.handleSeal)
	m.HandleFunc("POST /api/batches/{id}/anchors", s.handleAddAnchors)
	m.HandleFunc("GET /api/batches/{id}/anchors", s.handleListAnchors)
}

// ---- 公共工具 ----

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("httpapi: encode response: %v", err)
		}
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}

func decodeJSON(r *http.Request, dst interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return io.EOF
	}
	return json.Unmarshal(body, dst)
}

// pathID 解析路径通配符 {id} 为 int64。
func pathID(r *http.Request) (int64, bool) {
	return parseID(r.PathValue("id"))
}

// pathSID 解析路径通配符 {sid} 为 int64。
func pathSID(r *http.Request) (int64, bool) {
	return parseID(r.PathValue("sid"))
}
