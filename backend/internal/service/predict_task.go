package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"stock-forecast-backend/internal/model"
)

type PredictTaskStatus struct {
	TaskID    string                `json:"task_id"`
	Status    string                `json:"status"`
	Current   string                `json:"current_code,omitempty"`
	Done      int                   `json:"done"`
	Total     int                   `json:"total"`
	Results   []model.PredictResult `json:"results,omitempty"`
	Error     string                `json:"error,omitempty"`
	ExpiresAt time.Time             `json:"expires_at"`
}

type predictTask struct {
	id        string
	status    string
	canceled  bool
	requestID string
	current   string
	done      int
	total     int
	results   []model.PredictResult
	err       string
	createdAt time.Time
	expiresAt time.Time
}

var (
	predictTaskMu  sync.Mutex
	predictTasks   = make(map[string]*predictTask)
	requestTaskMap = make(map[string]string)
	predictTaskSem = make(chan struct{}, 3)
)

const predictTaskTTL = 30 * time.Minute

func CreatePredictTask(codes []string, period string, requestID string) (PredictTaskStatus, bool, error) {
	if len(codes) == 0 {
		return PredictTaskStatus{}, false, fmt.Errorf("请选择至少一只股票")
	}
	if period == "" {
		period = "daily"
	}
	requestID = strings.TrimSpace(requestID)

	now := time.Now()

	predictTaskMu.Lock()
	cleanupExpiredLocked(now)
	if requestID != "" {
		if existingID, ok := requestTaskMap[requestID]; ok {
			if t, ok2 := predictTasks[existingID]; ok2 && !t.expiresAt.IsZero() && now.Before(t.expiresAt) {
				out := buildPredictTaskStatus(t)
				predictTaskMu.Unlock()
				return out, false, nil
			}
			delete(requestTaskMap, requestID)
		}
	}
	predictTaskMu.Unlock()

	id := newTaskID()
	t := &predictTask{
		id:        id,
		status:    "pending",
		done:      0,
		total:     len(codes),
		results:   nil,
		err:       "",
		createdAt: now,
		expiresAt: now.Add(predictTaskTTL),
		requestID: requestID,
	}

	predictTaskMu.Lock()
	predictTasks[id] = t
	if requestID != "" {
		requestTaskMap[requestID] = id
	}
	predictTaskMu.Unlock()

	go runPredictTask(t, codes, period)
	return PredictTaskStatus{TaskID: id, Status: "pending", Done: 0, Total: len(codes), ExpiresAt: t.expiresAt}, true, nil
}

func GetPredictTaskStatus(taskID string) (PredictTaskStatus, bool) {
	now := time.Now()
	predictTaskMu.Lock()
	cleanupExpiredLocked(now)
	t, ok := predictTasks[taskID]
	if !ok {
		predictTaskMu.Unlock()
		return PredictTaskStatus{}, false
	}
	out := buildPredictTaskStatus(t)
	predictTaskMu.Unlock()
	return out, true
}

func CancelPredictTask(taskID string) (PredictTaskStatus, bool) {
	now := time.Now()
	predictTaskMu.Lock()
	cleanupExpiredLocked(now)
	t, ok := predictTasks[taskID]
	if !ok {
		predictTaskMu.Unlock()
		return PredictTaskStatus{}, false
	}

	switch t.status {
	case "done", "failed", "canceled":
		out := buildPredictTaskStatus(t)
		predictTaskMu.Unlock()
		return out, true
	default:
		t.canceled = true
		t.status = "canceled"
		t.err = "任务已取消"
		t.current = ""
		if t.requestID != "" {
			delete(requestTaskMap, t.requestID)
		}
		out := buildPredictTaskStatus(t)
		predictTaskMu.Unlock()
		return out, true
	}
}

func runPredictTask(t *predictTask, codes []string, period string) {
	predictTaskSem <- struct{}{}
	defer func() { <-predictTaskSem }()

	predictTaskMu.Lock()
	if t.status == "pending" {
		t.status = "running"
	}
	predictTaskMu.Unlock()

	results := make([]model.PredictResult, 0, len(codes))
	for i, code := range codes {
		predictTaskMu.Lock()
		if t.canceled {
			if t.status != "canceled" {
				t.status = "canceled"
				t.current = ""
				t.err = "任务已取消"
			}
			predictTaskMu.Unlock()
			return
		}
		t.current = code
		predictTaskMu.Unlock()

		res, err := predictSingleStock(code, period, false)
		if err != nil {
			predictTaskMu.Lock()
			t.status = "failed"
			t.done = i
			t.current = code
			t.results = results
			t.err = err.Error()
			predictTaskMu.Unlock()
			return
		}
		results = append(results, *res)

		predictTaskMu.Lock()
		if t.canceled {
			t.status = "canceled"
			t.current = ""
			t.err = "任务已取消"
			t.results = results
			if t.requestID != "" {
				delete(requestTaskMap, t.requestID)
			}
			predictTaskMu.Unlock()
			return
		}
		t.done = i + 1
		predictTaskMu.Unlock()
	}

	predictTaskMu.Lock()
	t.status = "done"
	t.results = results
	t.done = len(codes)
	t.current = ""
	if t.requestID != "" {
		delete(requestTaskMap, t.requestID)
	}
	predictTaskMu.Unlock()
}

func cleanupExpiredLocked(now time.Time) {
	for id, t := range predictTasks {
		if !t.expiresAt.IsZero() && now.After(t.expiresAt) {
			delete(predictTasks, id)
		}
	}
	for rid, tid := range requestTaskMap {
		t, ok := predictTasks[tid]
		if !ok || (!t.expiresAt.IsZero() && now.After(t.expiresAt)) {
			delete(requestTaskMap, rid)
		}
	}
}

func newTaskID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func buildPredictTaskStatus(t *predictTask) PredictTaskStatus {
	out := PredictTaskStatus{
		TaskID:    t.id,
		Status:    t.status,
		Current:   t.current,
		Done:      t.done,
		Total:     t.total,
		Error:     t.err,
		ExpiresAt: t.expiresAt,
	}
	if t.status == "done" || t.status == "failed" {
		out.Results = append([]model.PredictResult(nil), t.results...)
	}
	return out
}
