package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"task277-shellband/internal/diagnose"
	"task277-shellband/internal/model"
	"task277-shellband/internal/sample"
)

// parseID 将字符串解析为 int64；失败返回 (0, false)。
func parseID(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// mapError 将领域/存储错误映射为 HTTP 状态、错误码与消息。
func mapError(err error) (int, string, string) {
	if err == nil {
		return 0, "", ""
	}
	// 先匹配具体哨兵错误。
	switch {
	case errors.Is(err, model.ErrBatchNotFound),
		errors.Is(err, model.ErrBandNotFound),
		errors.Is(err, model.ErrSampleNotFound),
		errors.Is(err, model.ErrAnchorNotFound),
		errors.Is(err, model.ErrSnapshotNotFound):
		return http.StatusNotFound, "NOT_FOUND", err.Error()
	case err == model.ErrDuplicateCode,
		err == model.ErrSampleNoConflict,
		err == model.ErrAnchorConflict:
		return http.StatusConflict, "CONFLICT", err.Error()
	case errors.Is(err, model.ErrPositionOrder),
		errors.Is(err, model.ErrMissingUnit),
		errors.Is(err, model.ErrNoBands),
		errors.Is(err, model.ErrNoAnchors),
		errors.Is(err, model.ErrWrongStatus),
		errors.Is(err, model.ErrSealedImmutable),
		errors.Is(err, model.ErrInvalidTransition):
		return http.StatusBadRequest, "DOMAIN", err.Error()
	}
	// 领域错误（携带错误码）一律 400。
	var de *model.DomainError
	if errors.As(err, &de) {
		return http.StatusBadRequest, de.Code, de.Message
	}
	return http.StatusInternalServerError, "INTERNAL", err.Error()
}

func handleError(w http.ResponseWriter, err error) {
	status, code, msg := mapError(err)
	writeError(w, status, code, msg)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGlobalStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Stats()
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// ---- 批次 ----

type createBatchReq struct {
	Code    string `json:"code"`
	Species string `json:"species"`
}

func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var req createBatchReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	b, err := s.svc.CreateBatch(req.Code, req.Species)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	bs, err := s.svc.ListBatches()
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	b, err := s.svc.GetBatch(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type setSpeciesReq struct {
	Species string `json:"species"`
}

func (s *Server) handleSetSpecies(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req setSpeciesReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	if err := s.svc.SetSpecies(id, req.Species); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- 生长带 ----

type bandReq struct {
	BandIndex int     `json:"band_index"`
	StartPos  float64 `json:"start_pos"`
	EndPos    float64 `json:"end_pos"`
	Note      string  `json:"note"`
}

func (s *Server) handleAddBands(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var reqs []bandReq
	if err := decodeJSON(r, &reqs); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	bands := make([]*model.GrowthBand, 0, len(reqs))
	for _, q := range reqs {
		bands = append(bands, &model.GrowthBand{
			BandIndex: q.BandIndex,
			StartPos:  q.StartPos,
			EndPos:    q.EndPos,
			Note:      q.Note,
		})
	}
	out, err := s.svc.AddBands(id, bands)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListBands(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	bs, err := s.svc.ListBands(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

// ---- 采样 ----

type sampleReq struct {
	SampleNo        int64   `json:"sample_no"`
	RawPos          float64 `json:"raw_pos"`
	IsotopeValue    float64 `json:"isotope_value"`
	Unit            string  `json:"unit"`
	RecrystallScore float64 `json:"recrystall_score"`
}

func (s *Server) handleAddSamples(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var reqs []sampleReq
	if err := decodeJSON(r, &reqs); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	samples := make([]*model.IsotopeSample, 0, len(reqs))
	for _, q := range reqs {
		samples = append(samples, &model.IsotopeSample{
			SampleNo:        q.SampleNo,
			RawPos:          q.RawPos,
			IsotopeValue:    q.IsotopeValue,
			Unit:            q.Unit,
			RecrystallScore: q.RecrystallScore,
		})
	}
	out, dup, err := s.svc.AddSamples(id, samples)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"samples":        out,
		"duplicate_nums": dup,
	})
}

func (s *Server) handleListSamples(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	sm, err := s.svc.ListSamples(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sm)
}

// ---- 位置校正 ----

type correctReq struct {
	Shrinkage float64 `json:"shrinkage"`
	Method    string  `json:"method"`
}

func (s *Server) handleCorrect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req correctReq
	_ = decodeJSON(r, &req) // 空体使用默认参数
	corrs, err := s.svc.Correct(id, sample.CorrectionParams{Shrinkage: req.Shrinkage, Method: req.Method})
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"corrections": corrs,
		"count":       len(corrs),
	})
}

func (s *Server) handleListCorrections(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	cs, err := s.svc.ListCorrections(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// ---- 对齐 ----

func (s *Server) handleAlign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	res, err := s.svc.Align(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"gap_count":      res.GapCount(),
		"aligned_count":  res.AlignedCount(),
		"out_of_section": res.OutOfSection,
		"assignments":    res.Assignments,
		"band_kind":      res.BandKind,
	})
}

func (s *Server) handleListAlignments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	as, err := s.svc.ListAlignments(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, as)
}

// ---- 污染诊断 ----

type diagnoseReq struct {
	RecrystallThreshold float64 `json:"recrystall_threshold"`
	IsoExtreme          float64 `json:"iso_extreme"`
}

func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req diagnoseReq
	_ = decodeJSON(r, &req)
	opts := diagnose.Options{RecrystallThreshold: req.RecrystallThreshold, IsoExtreme: req.IsoExtreme}
	res, err := s.svc.Diagnose(id, opts)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- 污染裁决 ----

type verdictReq struct {
	SampleID int64  `json:"sample_id"`
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason"`
	Reviewer string `json:"reviewer"`
}

func (s *Server) handleRecordVerdicts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var reqs []verdictReq
	if err := decodeJSON(r, &reqs); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	vs := make([]*model.PollutionVerdict, 0, len(reqs))
	for _, q := range reqs {
		vs = append(vs, &model.PollutionVerdict{
			SampleID: q.SampleID,
			Verdict:  q.Verdict,
			Reason:   q.Reason,
			Reviewer: q.Reviewer,
		})
	}
	if err := s.svc.RecordVerdicts(id, vs); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListVerdicts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	vs, err := s.svc.ListVerdicts(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

// ---- 季节快照 ----

type snapshotReq struct {
	Publish bool `json:"publish"`
}

func (s *Server) handleBuildSnapshot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req snapshotReq
	_ = decodeJSON(r, &req)
	snap, err := s.svc.BuildSnapshot(id, req.Publish)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	snaps, err := s.svc.ListSnapshots(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	sid, ok := pathSID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_SID", "invalid snapshot id")
		return
	}
	_ = id
	snap, err := s.svc.GetSnapshot(sid)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

type publishReq struct {
	SnapshotID int64 `json:"snapshot_id"`
}

func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req publishReq
	_ = decodeJSON(r, &req)
	if req.SnapshotID == 0 {
		// 未指定则发布该批次最新草稿。
		snaps, err := s.svc.ListSnapshots(id)
		if err != nil {
			handleError(w, err)
			return
		}
		for i := len(snaps) - 1; i >= 0; i-- {
			if snaps[i].Status == model.SnapshotDraft {
				req.SnapshotID = snaps[i].ID
				break
			}
		}
	}
	if req.SnapshotID == 0 {
		writeError(w, http.StatusBadRequest, "NO_DRAFT", "no draft snapshot to publish")
		return
	}
	if err := s.svc.PublishSnapshot(req.SnapshotID); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- 状态流转 ----

type statusReq struct {
	Status string `json:"status"`
}

func (s *Server) handleTransitionStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var req statusReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	if err := s.svc.TransitionStatus(id, model.BatchStatus(req.Status)); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSeal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	if err := s.svc.Seal(id); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- 年代锚点 ----

type anchorReq struct {
	Position float64 `json:"position"`
	AgeYear  float64 `json:"age_year"`
	Source   string  `json:"source"`
}

func (s *Server) handleAddAnchors(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	var reqs []anchorReq
	if err := decodeJSON(r, &reqs); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "invalid request body")
		return
	}
	as := make([]*model.AgeAnchor, 0, len(reqs))
	for _, q := range reqs {
		as = append(as, &model.AgeAnchor{
			Position: q.Position,
			AgeYear:  q.AgeYear,
			Source:   q.Source,
		})
	}
	if err := s.svc.AddAnchors(id, as); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleListAnchors(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "invalid batch id")
		return
	}
	as, err := s.svc.ListAnchors(id)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, as)
}
