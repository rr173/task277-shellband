package model

import "errors"

// 领域错误集合。所有错误均携带稳定错误码，便于 API 层映射为可读响应。
var (
	// ErrBatchNotFound 批次不存在。
	ErrBatchNotFound = errors.New("model: batch not found")
	// ErrBandNotFound 生长带不存在。
	ErrBandNotFound = errors.New("model: growth band not found")
	// ErrSampleNotFound 采样点不存在。
	ErrSampleNotFound = errors.New("model: isotope sample not found")
	// ErrAnchorNotFound 年代锚点不存在。
	ErrAnchorNotFound = errors.New("model: age anchor not found")
	// ErrSnapshotNotFound 季节快照不存在。
	ErrSnapshotNotFound = errors.New("model: seasonal snapshot not found")

	// ErrPositionOrder 采样点校正后位置倒序（违反单调不降不变量）。
	ErrPositionOrder = errors.New("model: corrected positions are out of order")
	// ErrMissingUnit 同位素单位缺失。
	ErrMissingUnit = errors.New("model: isotope unit is missing")
	// ErrAnchorConflict 年代锚点位置冲突（同位置重复锚点）。
	ErrAnchorConflict = errors.New("model: age anchor position conflict")
	// ErrSealedImmutable 封存批次不可修改。
	ErrSealedImmutable = errors.New("model: sealed batch is immutable")
	// ErrSampleNoConflict 采样编号冲突（幂等编号重复）。
	ErrSampleNoConflict = errors.New("model: sample number conflict (not idempotent)")
	// ErrDuplicateCode 批次编码重复。
	ErrDuplicateCode = errors.New("model: batch code already exists")
	// ErrInvalidTransition 非法状态流转。
	ErrInvalidTransition = errors.New("model: invalid batch status transition")
	// ErrNoBands 批次缺少生长带边界，无法对齐。
	ErrNoBands = errors.New("model: no growth bands defined for batch")
	// ErrNoAnchors 快照需要年代锚点但缺失。
	ErrNoAnchors = errors.New("model: no age anchors defined for batch")
	// ErrWrongStatus 当前批次状态不支持该操作。
	ErrWrongStatus = errors.New("model: operation not allowed in current batch status")
)

// DomainError 携带错误码的领域错误，便于 HTTP 层映射。
type DomainError struct {
	Code    string
	Message string
	Err     error
}

// Error 实现 error 接口。
func (e *DomainError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + " (" + e.Err.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

// Unwrap 支持 errors.Is 链。
func (e *DomainError) Unwrap() error { return e.Err }

// NewDomainError 构造领域错误。
func NewDomainError(code, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err}
}
